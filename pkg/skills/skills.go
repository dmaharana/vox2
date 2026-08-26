package skills

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"go-harness/pkg/tools"

	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// Agent Skills specification limits
const (
	MaxNameLength        = 64
	MaxDescriptionLength = 1024
)

var (
	validNameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)
)

// SkillDiagnostic represents a warning, validation issue, or collision encountered during skill discovery.
type SkillDiagnostic struct {
	Type      string `json:"type"`      // "warning", "collision"
	Message   string `json:"message"`   // Human-readable diagnostic message
	Path      string `json:"path"`      // File path where issue was found
	SkillName string `json:"skill_name,omitempty"`
}

// SkillFile represents an auxiliary file discovered within a skill directory.
type SkillFile struct {
	Path      string `json:"path"`      // Relative path within skill folder (e.g. "scripts/deploy.sh")
	Category  string `json:"category"`  // "script", "reference", "template", "asset", "other"
	SizeBytes int64  `json:"size_bytes"`
}

// Skill represents a loaded skill capability following the Agent Skills standard.
type Skill struct {
	Name                   string            `json:"name"`
	Description            string            `json:"description"`
	Path                   string            `json:"path"` // Absolute/relative path to SKILL.md or .md file
	Directory              string            `json:"directory,omitempty"`
	Files                  []SkillFile       `json:"files,omitempty"`
	Content                string            `json:"content"`
	Enabled                bool              `json:"enabled"`
	DisableModelInvocation bool              `json:"disable_model_invocation,omitempty"`
	License                string            `json:"license,omitempty"`
	Compatibility          string            `json:"compatibility,omitempty"`
	Metadata               map[string]string `json:"metadata,omitempty"`
	AllowedTools           string            `json:"allowed_tools,omitempty"`
	Source                 string            `json:"source,omitempty"` // "project", "user", "custom"
}

// DirSource associates a directory path with its origin scope.
type DirSource struct {
	Dir    string `json:"dir"`
	Source string `json:"source"` // "project", "user", "custom"
}

// Loader manages discovery, validation, diagnostics, and lifecycle of skills.
type Loader struct {
	mu          sync.RWMutex
	dirSources  []DirSource
	skills      map[string]*Skill
	diagnostics []SkillDiagnostic
}

// NewLoader creates a new skill loader. If base dirs are provided, they are registered with priority.
func NewLoader(dirs ...string) *Loader {
	loader := &Loader{
		skills:      make(map[string]*Skill),
		diagnostics: make([]SkillDiagnostic, 0),
	}

	for _, d := range dirs {
		if strings.TrimSpace(d) != "" {
			loader.dirSources = append(loader.dirSources, DirSource{
				Dir:    d,
				Source: "project",
			})
		}
	}

	// Register default standard locations if not explicitly configured
	if len(loader.dirSources) == 0 {
		loader.dirSources = append(loader.dirSources, DirSource{Dir: "./skills", Source: "project"})
	}

	// Add standard .agents/skills project location
	loader.AddDirectory(".agents/skills", "project")

	// Add global user skills if HOME directory is available
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		loader.AddDirectory(filepath.Join(home, ".vox", "skills"), "user")
		loader.AddDirectory(filepath.Join(home, ".agents", "skills"), "user")
		loader.AddDirectory(filepath.Join(home, ".pi", "agent", "skills"), "user")
	}

	return loader
}

// AddDirectory adds an additional directory to scan for skills.
func (l *Loader) AddDirectory(dir string, source string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cleaned := filepath.Clean(dir)
	for _, existing := range l.dirSources {
		if filepath.Clean(existing.Dir) == cleaned {
			return
		}
	}
	l.dirSources = append(l.dirSources, DirSource{
		Dir:    cleaned,
		Source: source,
	})
}

// Load scans all configured directories and loads all skills.
func (l *Loader) Load() error {
	return l.Reload()
}

// Reload rescans all configured directories and refreshes all skills and diagnostics.
func (l *Loader) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	newSkills := make(map[string]*Skill)
	loadedRealPaths := make(map[string]bool)
	var diagnostics []SkillDiagnostic

	for _, ds := range l.dirSources {
		info, err := os.Stat(ds.Dir)
		if os.IsNotExist(err) || !info.IsDir() {
			continue
		}

		discoveredSkills, diags := l.scanDirectory(ds.Dir, ds.Source, true)
		diagnostics = append(diagnostics, diags...)

		for _, s := range discoveredSkills {
			realPath, err := filepath.EvalSymlinks(s.Path)
			if err != nil {
				realPath = s.Path
			}
			if loadedRealPaths[realPath] {
				// Duplicate file already loaded (e.g. symlink)
				continue
			}

			if existing, exists := newSkills[s.Name]; exists {
				diagnostics = append(diagnostics, SkillDiagnostic{
					Type:      "collision",
					Message:   fmt.Sprintf("skill name %q collision: keeping %s (%s), skipping %s (%s)", s.Name, existing.Path, existing.Source, s.Path, s.Source),
					Path:      s.Path,
					SkillName: s.Name,
				})
				continue
			}

			// Preserve existing enabled state if reloaded
			if prev, ok := l.skills[s.Name]; ok {
				s.Enabled = prev.Enabled
			} else {
				s.Enabled = true
			}

			newSkills[s.Name] = s
			loadedRealPaths[realPath] = true
		}
	}

	l.skills = newSkills
	l.diagnostics = diagnostics
	log.Info().Int("skills_count", len(l.skills)).Int("diagnostics_count", len(l.diagnostics)).Msg("Loaded skills from disk")
	return nil
}

func (l *Loader) scanDirectory(dir, source string, isRoot bool) ([]*Skill, []SkillDiagnostic) {
	var skillsList []*Skill
	var diagnostics []SkillDiagnostic

	entries, err := os.ReadDir(dir)
	if err != nil {
		return skillsList, diagnostics
	}

	// 1. First check if current directory is a skill root containing SKILL.md
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), "SKILL.md") {
			skillPath := filepath.Join(dir, entry.Name())
			skill, diags, err := parseSkillFile(skillPath, filepath.Base(dir), source)
			diagnostics = append(diagnostics, diags...)
			if err == nil && skill != nil {
				skillsList = append(skillsList, skill)
			}
			// Directory is a skill root; do not recurse into child directories as separate skills
			return skillsList, diagnostics
		}
	}

	// 2. Scan subdirectories and direct .md files
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" {
			continue
		}

		fullPath := filepath.Join(dir, name)
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}

		if fileInfo.IsDir() {
			subSkills, subDiags := l.scanDirectory(fullPath, source, false)
			skillsList = append(skillsList, subSkills...)
			diagnostics = append(diagnostics, subDiags...)
		} else if isRoot && strings.HasSuffix(strings.ToLower(name), ".md") {
			// Direct root .md files
			skillName := strings.TrimSuffix(name, filepath.Ext(name))
			skill, diags, err := parseSkillFile(fullPath, skillName, source)
			diagnostics = append(diagnostics, diags...)
			if err == nil && skill != nil {
				skillsList = append(skillsList, skill)
			}
		}
	}

	return skillsList, diagnostics
}

// List returns a list of all loaded skills.
func (l *Loader) List() []Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	list := make([]Skill, 0, len(l.skills))
	for _, s := range l.skills {
		list = append(list, *s)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return list
}

// Diagnostics returns all validation warnings and collision diagnostics.
func (l *Loader) Diagnostics() []SkillDiagnostic {
	l.mu.RLock()
	defer l.mu.RUnlock()

	diagList := make([]SkillDiagnostic, len(l.diagnostics))
	copy(diagList, l.diagnostics)
	return diagList
}

// Get retrieves a skill by name.
func (l *Loader) Get(name string) (*Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	s, ok := l.skills[name]
	if !ok {
		// Case-insensitive lookup fallback
		for _, item := range l.skills {
			if strings.EqualFold(item.Name, name) {
				cpy := *item
				return &cpy, true
			}
		}
		return nil, false
	}
	cpy := *s
	return &cpy, true
}

// SetEnabled enables or disables a skill by name.
func (l *Loader) SetEnabled(name string, enabled bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	s, ok := l.skills[name]
	if !ok {
		for _, item := range l.skills {
			if strings.EqualFold(item.Name, name) {
				item.Enabled = enabled
				return true
			}
		}
		return false
	}
	s.Enabled = enabled
	return true
}

// BuildPromptSection generates the progressive disclosure system prompt section adhering to the Agent Skills standard XML specification.
// Skills with DisableModelInvocation=true are excluded from the prompt.
func (l *Loader) BuildPromptSection() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var visible []*Skill
	for _, s := range l.skills {
		if s.Enabled && !s.DisableModelInvocation {
			visible = append(visible, s)
		}
	}

	if len(visible) == 0 {
		return ""
	}

	sort.Slice(visible, func(i, j int) bool {
		return visible[i].Name < visible[j].Name
	})

	var sb strings.Builder
	sb.WriteString("\nThe following skills provide specialized instructions for specific tasks.\n")
	sb.WriteString("Use the read_skill tool (or read_file on the skill location) to load a skill's full instructions when the task matches its description.\n")
	sb.WriteString("When a skill file references a relative path, resolve it against the skill directory (parent of SKILL.md / dirname of the path) and use that absolute path in tool commands.\n\n")
	sb.WriteString("<available_skills>\n")

	for _, s := range visible {
		sb.WriteString("  <skill>\n")
		sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", escapeXML(s.Name)))
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", escapeXML(s.Description)))
		sb.WriteString(fmt.Sprintf("    <location>%s</location>\n", escapeXML(s.Path)))
		sb.WriteString("  </skill>\n")
	}

	sb.WriteString("</available_skills>\n")

	return sb.String()
}

// RegisterSkillTools registers skill-related tools into the agent tool registry.
func (l *Loader) RegisterSkillTools(reg *tools.Registry) {
	reg.Register(l.newReadSkillTool())
}

func (l *Loader) newReadSkillTool() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "read_skill",
		Description: "Loads and retrieves the full domain instructions, workflow guidelines, runbooks, and companion scripts/references for a specific named skill.",
		Category:    "builtin",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {
					Type:        jsonschema.String,
					Description: "The exact name of the skill to load and read (e.g. 'code-review')",
				},
			},
			Required: []string{"name"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}
			if strings.TrimSpace(in.Name) == "" {
				return nil, fmt.Errorf("skill name is required")
			}

			s, ok := l.Get(strings.TrimSpace(in.Name))
			if !ok {
				return nil, fmt.Errorf("skill '%s' not found", in.Name)
			}
			if !s.Enabled {
				return nil, fmt.Errorf("skill '%s' is currently disabled", in.Name)
			}

			files := s.Files
			if files == nil {
				files = []SkillFile{}
			}

			return map[string]any{
				"name":                     s.Name,
				"description":              s.Description,
				"content":                  s.Content,
				"path":                     s.Path,
				"directory":                s.Directory,
				"available_files":          files,
				"disable_model_invocation": s.DisableModelInvocation,
				"license":                  s.License,
				"compatibility":            s.Compatibility,
				"metadata":                 s.Metadata,
				"allowed_tools":            s.AllowedTools,
				"source":                   s.Source,
			}, nil
		},
	}
}

// Validation helpers per Agent Skills standard
func validateSkillName(name string) []string {
	var errs []string
	if len(name) > MaxNameLength {
		errs = append(errs, fmt.Sprintf("name exceeds %d characters (%d)", MaxNameLength, len(name)))
	}
	if !validNameRegex.MatchString(name) {
		errs = append(errs, "name contains invalid characters (must be lowercase a-z, 0-9, hyphens only)")
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		errs = append(errs, "name must not start or end with a hyphen")
	}
	if strings.Contains(name, "--") {
		errs = append(errs, "name must not contain consecutive hyphens")
	}
	return errs
}

func validateSkillDescription(desc string) []string {
	var errs []string
	trimmed := strings.TrimSpace(desc)
	if trimmed == "" {
		errs = append(errs, "description is required")
	} else if len(desc) > MaxDescriptionLength {
		errs = append(errs, fmt.Sprintf("description exceeds %d characters (%d)", MaxDescriptionLength, len(desc)))
	}
	return errs
}

func parseSkillFile(filePath, defaultName, source string) (*Skill, []SkillDiagnostic, error) {
	var diagnostics []SkillDiagnostic

	data, err := os.ReadFile(filePath)
	if err != nil {
		diagnostics = append(diagnostics, SkillDiagnostic{
			Type:    "warning",
			Message: fmt.Sprintf("failed to read skill file: %v", err),
			Path:    filePath,
		})
		return nil, diagnostics, err
	}

	content := string(data)
	isDeclaredSkill := strings.EqualFold(filepath.Base(filePath), "SKILL.md")

	name := defaultName
	description := ""
	disableModelInvocation := false
	license := ""
	compatibility := ""
	allowedTools := ""
	metadata := make(map[string]string)

	// Parse YAML frontmatter
	if strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			body := strings.TrimSpace(parts[2])
			content = body

			scanner := bufio.NewScanner(bytes.NewReader([]byte(frontmatter)))
			inMetadataBlock := false

			for scanner.Scan() {
				rawLine := scanner.Text()
				line := strings.TrimSpace(rawLine)

				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				if strings.HasPrefix(line, "metadata:") {
					inMetadataBlock = true
					continue
				}

				if inMetadataBlock {
					if strings.HasPrefix(rawLine, "  ") || strings.HasPrefix(rawLine, "\t") {
						kvParts := strings.SplitN(line, ":", 2)
						if len(kvParts) == 2 {
							k := strings.TrimSpace(kvParts[0])
							v := strings.TrimSpace(strings.Trim(kvParts[1], `"'`))
							metadata[k] = v
						}
						continue
					}
					inMetadataBlock = false
				}

				if strings.HasPrefix(line, "name:") {
					val := strings.TrimSpace(strings.Trim(strings.TrimPrefix(line, "name:"), `"'`))
					if val != "" {
						name = val
					}
				} else if strings.HasPrefix(line, "description:") {
					val := strings.TrimSpace(strings.Trim(strings.TrimPrefix(line, "description:"), `"'`))
					description = val
				} else if strings.HasPrefix(line, "disable-model-invocation:") || strings.HasPrefix(line, "disable_model_invocation:") {
					val := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "disable-model-invocation:"), "disable_model_invocation:")))
					disableModelInvocation = (val == "true" || val == "yes" || val == "1")
				} else if strings.HasPrefix(line, "license:") {
					license = strings.TrimSpace(strings.Trim(strings.TrimPrefix(line, "license:"), `"'`))
				} else if strings.HasPrefix(line, "compatibility:") {
					compatibility = strings.TrimSpace(strings.Trim(strings.TrimPrefix(line, "compatibility:"), `"'`))
				} else if strings.HasPrefix(line, "allowed-tools:") || strings.HasPrefix(line, "allowed_tools:") {
					allowedTools = strings.TrimSpace(strings.Trim(strings.TrimPrefix(strings.TrimPrefix(line, "allowed-tools:"), "allowed_tools:"), `"'`))
				}
			}
		}
	}

	// Fallback description resolution for standalone markdown files
	if description == "" {
		if isDeclaredSkill {
			diagnostics = append(diagnostics, SkillDiagnostic{
				Type:      "warning",
				Message:   "description is required",
				Path:      filePath,
				SkillName: name,
			})
			return nil, diagnostics, fmt.Errorf("description is required in %s", filePath)
		}

		// For standalone .md files: ignore standard documentation files
		baseLower := strings.ToLower(filepath.Base(filePath))
		if baseLower == "readme.md" || baseLower == "agents.md" || baseLower == "claude.md" ||
			baseLower == "license.md" || baseLower == "contributing.md" || baseLower == "changelog.md" {
			return nil, diagnostics, nil
		}

		// Extract first non-heading non-empty line as description, or fallback to default
		lines := strings.Split(content, "\n")
		for _, l := range lines {
			trimmed := strings.TrimSpace(strings.TrimPrefix(l, "#"))
			if trimmed != "" && !strings.EqualFold(trimmed, name) {
				description = trimmed
				break
			}
		}
		if description == "" {
			description = fmt.Sprintf("Domain instructions for %s", name)
		}
	}

	// Validation
	descErrors := validateSkillDescription(description)
	for _, e := range descErrors {
		diagnostics = append(diagnostics, SkillDiagnostic{
			Type:      "warning",
			Message:   e,
			Path:      filePath,
			SkillName: name,
		})
	}

	nameErrors := validateSkillName(name)
	for _, e := range nameErrors {
		diagnostics = append(diagnostics, SkillDiagnostic{
			Type:      "warning",
			Message:   e,
			Path:      filePath,
			SkillName: name,
		})
	}

	skillDir := filepath.Dir(filePath)
	var files []SkillFile
	if isDeclaredSkill {
		files = discoverSkillFiles(skillDir)
	} else {
		files = []SkillFile{}
	}

	return &Skill{
		Name:                   name,
		Description:            description,
		Path:                   filePath,
		Directory:              skillDir,
		Files:                  files,
		Content:                content,
		Enabled:                true,
		DisableModelInvocation: disableModelInvocation,
		License:                license,
		Compatibility:          compatibility,
		Metadata:               metadata,
		AllowedTools:           allowedTools,
		Source:                 source,
	}, diagnostics, nil
}

func discoverSkillFiles(skillDir string) []SkillFile {
	var files []SkillFile
	_ = filepath.Walk(skillDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}

		rel, err := filepath.Rel(skillDir, path)
		if err != nil || rel == "." {
			return nil
		}

		rel = filepath.ToSlash(rel)

		// Ignore hidden files and directories
		parts := strings.Split(rel, "/")
		for _, p := range parts {
			if strings.HasPrefix(p, ".") || p == "node_modules" || p == "__pycache__" {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			return nil
		}

		// Skip the root SKILL.md since it is already returned in content
		if strings.EqualFold(rel, "SKILL.md") {
			return nil
		}

		category := categorizeSkillFile(rel)
		files = append(files, SkillFile{
			Path:      rel,
			Category:  category,
			SizeBytes: info.Size(),
		})
		return nil
	})

	if files == nil {
		files = []SkillFile{}
	}
	return files
}

func categorizeSkillFile(relPath string) string {
	lower := strings.ToLower(relPath)
	ext := filepath.Ext(lower)

	if strings.HasPrefix(lower, "scripts/") || strings.HasPrefix(lower, "bin/") ||
		ext == ".sh" || ext == ".py" || ext == ".js" || ext == ".ts" || ext == ".bash" || ext == ".zsh" || ext == ".rb" || ext == ".go" {
		return "script"
	}
	if strings.HasPrefix(lower, "templates/") || strings.HasPrefix(lower, "tpl/") || strings.HasSuffix(lower, ".template") || strings.HasSuffix(lower, ".tmpl") {
		return "template"
	}
	if strings.HasPrefix(lower, "references/") || strings.HasPrefix(lower, "ref/") || strings.HasPrefix(lower, "docs/") ||
		ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".md" || ext == ".txt" || ext == ".csv" || ext == ".xml" || ext == ".proto" {
		return "reference"
	}
	if strings.HasPrefix(lower, "assets/") || strings.HasPrefix(lower, "images/") ||
		ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".svg" || ext == ".gif" || ext == ".ico" {
		return "asset"
	}
	return "other"
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

package skills

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go-harness/pkg/tools"

	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// SkillFile represents an auxiliary file discovered within a skill directory.
type SkillFile struct {
	Path      string `json:"path"`      // Relative path within skill folder (e.g. "scripts/deploy.sh")
	Category  string `json:"category"`  // "script", "reference", "template", "asset", "other"
	SizeBytes int64  `json:"size_bytes"`
}

// Skill represents a loaded skill capability.
type Skill struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Path        string      `json:"path"`
	Directory   string      `json:"directory,omitempty"`
	Files       []SkillFile `json:"files,omitempty"`
	Content     string      `json:"content"`
	Enabled     bool        `json:"enabled"`
}

// Loader manages discovery and state of skills from the filesystem.
type Loader struct {
	mu     sync.RWMutex
	dir    string
	skills map[string]*Skill
}

// NewLoader creates a new skill loader pointing to a base directory.
func NewLoader(dir string) *Loader {
	if dir == "" {
		dir = "./skills"
	}
	return &Loader{
		dir:    dir,
		skills: make(map[string]*Skill),
	}
}

// Load scans the configured directory and loads all skills.
func (l *Loader) Load() error {
	return l.Reload()
}

// Reload scans the configured directory and refreshes all skills from disk.
func (l *Loader) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := os.Stat(l.dir); os.IsNotExist(err) {
		// Create skills directory if missing
		_ = os.MkdirAll(l.dir, 0755)
		l.skills = make(map[string]*Skill)
		return nil
	}

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	newSkills := make(map[string]*Skill)
	for _, entry := range entries {
		if entry.IsDir() {
			skillPath := filepath.Join(l.dir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillPath); err == nil {
				skill, err := parseSkillFile(skillPath, entry.Name())
				if err != nil {
					log.Warn().Err(err).Str("path", skillPath).Msg("Failed to parse skill")
					continue
				}
				// Preserve existing enabled state if reloaded
				if existing, ok := l.skills[skill.Name]; ok {
					skill.Enabled = existing.Enabled
				} else {
					skill.Enabled = true
				}
				newSkills[skill.Name] = skill
			}
		} else if strings.HasSuffix(entry.Name(), ".md") {
			skillPath := filepath.Join(l.dir, entry.Name())
			skillName := strings.TrimSuffix(entry.Name(), ".md")
			skill, err := parseSkillFile(skillPath, skillName)
			if err == nil {
				if existing, ok := l.skills[skill.Name]; ok {
					skill.Enabled = existing.Enabled
				} else {
					skill.Enabled = true
				}
				newSkills[skill.Name] = skill
			}
		}
	}

	l.skills = newSkills
	log.Info().Int("count", len(l.skills)).Str("dir", l.dir).Msg("Loaded skills from disk")
	return nil
}

// List returns a list of all skills.
func (l *Loader) List() []Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	list := make([]Skill, 0, len(l.skills))
	for _, s := range l.skills {
		list = append(list, *s)
	}
	return list
}

// Get retrieves a skill by name.
func (l *Loader) Get(name string) (*Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	s, ok := l.skills[name]
	if !ok {
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
		return false
	}
	s.Enabled = enabled
	return true
}

// BuildPromptSection generates a formatted system prompt segment listing active skills (metadata index for progressive disclosure).
func (l *Loader) BuildPromptSection() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var active []*Skill
	for _, s := range l.skills {
		if s.Enabled {
			active = append(active, s)
		}
	}

	if len(active) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Available Skills (Progressive Disclosure)\n")
	sb.WriteString("You have access to specialized domain skills listed below. Only lightweight metadata is shown.\n")
	sb.WriteString("When a task matches or requires a skill, or when instructed by the user, invoke the `read_skill` tool with the skill's `name` to load its full step-by-step instructions, runbooks, and companion scripts/references:\n\n")

	for _, s := range active {
		fileSummary := ""
		if len(s.Files) > 0 {
			fileSummary = fmt.Sprintf(" (%d helper files available: scripts, references, templates)", len(s.Files))
		}
		sb.WriteString(fmt.Sprintf("- **%s**: %s%s\n", s.Name, s.Description, fileSummary))
	}

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
				// Try case-insensitive lookup
				for _, item := range l.List() {
					if strings.EqualFold(item.Name, strings.TrimSpace(in.Name)) {
						s = &item
						ok = true
						break
					}
				}
			}

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
				"name":            s.Name,
				"description":     s.Description,
				"content":         s.Content,
				"path":            s.Path,
				"directory":       s.Directory,
				"available_files": files,
			}, nil
		},
	}
}

func parseSkillFile(filePath, defaultName string) (*Skill, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	name := defaultName
	description := ""

	// Check for YAML frontmatter
	if strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			body := strings.TrimSpace(parts[2])

			scanner := bufio.NewScanner(bytes.NewReader([]byte(frontmatter)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "name:") {
					name = strings.TrimSpace(strings.Trim(strings.TrimPrefix(line, "name:"), `"'`))
				} else if strings.HasPrefix(line, "description:") {
					description = strings.TrimSpace(strings.Trim(strings.TrimPrefix(line, "description:"), `"'`))
				}
			}
			content = body
		}
	}

	if description == "" {
		description = fmt.Sprintf("Domain instructions for %s", name)
	}

	var skillDir string
	var files []SkillFile

	if strings.EqualFold(filepath.Base(filePath), "SKILL.md") {
		skillDir = filepath.Dir(filePath)
		files = discoverSkillFiles(skillDir)
	} else {
		skillDir = filepath.Dir(filePath)
		files = []SkillFile{}
	}

	return &Skill{
		Name:        name,
		Description: description,
		Path:        filePath,
		Directory:   skillDir,
		Files:       files,
		Content:     content,
		Enabled:     true,
	}, nil
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

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

// Skill represents a loaded skill capability.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	Enabled     bool   `json:"enabled"`
}

// Loader manages discovery and state of skills from the filesystem.
type Loader struct {
	mu       sync.RWMutex
	dir      string
	skills   map[string]*Skill
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
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := os.Stat(l.dir); os.IsNotExist(err) {
		// Create skills directory if missing
		_ = os.MkdirAll(l.dir, 0755)
		return nil
	}

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

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
				l.skills[skill.Name] = skill
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
				l.skills[skill.Name] = skill
			}
		}
	}

	log.Info().Int("count", len(l.skills)).Str("dir", l.dir).Msg("Loaded skills")
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
	sb.WriteString("When a task matches or requires a skill, or when instructed by the user, invoke the `read_skill` tool with the skill's `name` to load its full step-by-step instructions and runbooks before executing the task:\n\n")

	for _, s := range active {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Name, s.Description))
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
		Description: "Loads and retrieves the full domain instructions, workflow guidelines, and runbooks for a specific named skill.",
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

			return map[string]any{
				"name":        s.Name,
				"description": s.Description,
				"content":     s.Content,
				"path":        s.Path,
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

	return &Skill{
		Name:        name,
		Description: description,
		Path:        filePath,
		Content:     content,
		Enabled:     true,
	}, nil
}

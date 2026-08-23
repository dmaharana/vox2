package skills

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
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

// BuildPromptSection generates a formatted system prompt segment listing active skills.
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
	sb.WriteString("\n## Available Skills\n")
	sb.WriteString("You have access to specialized domain skills. Follow their guidelines when relevant:\n\n")

	for _, s := range active {
		sb.WriteString(fmt.Sprintf("### Skill: %s\n", s.Name))
		if s.Description != "" {
			sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", s.Description))
		}
		sb.WriteString(s.Content)
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
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

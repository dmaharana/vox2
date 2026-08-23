package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsLoader(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create skill 1 in subfolder
	skill1Dir := filepath.Join(tempDir, "code-review")
	_ = os.MkdirAll(skill1Dir, 0755)
	skill1Content := `---
name: code-review
description: Expert Go code reviewer
---
Review code for idiomatic Go conventions, error handling, and performance.`
	_ = os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(skill1Content), 0644)

	// Create skill 2 as standalone .md file
	skill2Content := `# Weather Skill
Helps with weather forecasting and radar alerts.`
	_ = os.WriteFile(filepath.Join(tempDir, "weather.md"), []byte(skill2Content), 0644)

	loader := NewLoader(tempDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("failed to load skills: %v", err)
	}

	skills := loader.List()
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	s1, ok := loader.Get("code-review")
	if !ok || s1.Description != "Expert Go code reviewer" {
		t.Errorf("skill 1 parse mismatch: %+v", s1)
	}

	promptSection := loader.BuildPromptSection()
	if !strings.Contains(promptSection, "Available Skills") || !strings.Contains(promptSection, "code-review") {
		t.Errorf("expected prompt section to contain skills: %s", promptSection)
	}

	// Test disable skill
	loader.SetEnabled("code-review", false)
	updatedPrompt := loader.BuildPromptSection()
	if strings.Contains(updatedPrompt, "code-review") {
		t.Errorf("disabled skill should not be in prompt section")
	}
}

package skills

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-harness/pkg/tools"
)

func TestSkillsLoaderAndTools(t *testing.T) {
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
Detailed guidelines: Review code for idiomatic Go conventions, error handling, and performance.`
	_ = os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(skill1Content), 0644)

	// Create skill 2 as standalone .md file
	skill2Content := `# Weather Skill
Helps with weather forecasting and radar alerts.`
	_ = os.WriteFile(filepath.Join(tempDir, "weather.md"), []byte(skill2Content), 0644)

	loader := NewLoader(tempDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("failed to load skills: %v", err)
	}

	skillsList := loader.List()
	if len(skillsList) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skillsList))
	}

	s1, ok := loader.Get("code-review")
	if !ok || s1.Description != "Expert Go code reviewer" {
		t.Errorf("skill 1 parse mismatch: %+v", s1)
	}

	promptSection := loader.BuildPromptSection()
	if !strings.Contains(promptSection, "Available Skills") || !strings.Contains(promptSection, "code-review") {
		t.Errorf("expected prompt section to contain skills: %s", promptSection)
	}
	// Verify progressive disclosure: bulk content should NOT be in the system prompt section
	if strings.Contains(promptSection, "Detailed guidelines:") {
		t.Errorf("progressive disclosure violation: full content should not be in system prompt section")
	}

	// Register tools
	reg := tools.NewRegistry()
	loader.RegisterSkillTools(reg)

	toolDef, exists := reg.Get("read_skill")
	if !exists {
		t.Fatalf("expected read_skill tool to be registered")
	}
	if toolDef.Category != "builtin" {
		t.Errorf("expected category builtin, got %s", toolDef.Category)
	}

	// Execute read_skill tool for code-review
	argsJSON, _ := json.Marshal(map[string]string{"name": "code-review"})
	res, err := reg.Execute(context.Background(), "read_skill", argsJSON)
	if err != nil {
		t.Fatalf("unexpected error reading skill: %v", err)
	}
	resMap, ok := res.(map[string]any)
	if !ok || !strings.Contains(resMap["content"].(string), "Detailed guidelines:") {
		t.Errorf("expected skill content in result: %+v", res)
	}

	// Test disable skill
	loader.SetEnabled("code-review", false)
	updatedPrompt := loader.BuildPromptSection()
	if strings.Contains(updatedPrompt, "code-review") {
		t.Errorf("disabled skill should not be in prompt section")
	}

	// Executing read_skill on disabled skill should return error
	_, err = reg.Execute(context.Background(), "read_skill", argsJSON)
	if err == nil {
		t.Errorf("expected error reading disabled skill, got nil")
	}

	// Executing read_skill on unknown skill
	badArgs, _ := json.Marshal(map[string]string{"name": "non-existent"})
	_, err = reg.Execute(context.Background(), "read_skill", badArgs)
	if err == nil {
		t.Errorf("expected error reading unknown skill, got nil")
	}
}

func TestSkillsDynamicReloadAndCompanionManifest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "skills-manifest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	loader := NewLoader(tempDir)
	_ = loader.Load()

	// 1. Create a complete multi-file skill folder
	skillDir := filepath.Join(tempDir, "k8s-deploy")
	_ = os.MkdirAll(filepath.Join(skillDir, "scripts"), 0755)
	_ = os.MkdirAll(filepath.Join(skillDir, "references"), 0755)
	_ = os.MkdirAll(filepath.Join(skillDir, "templates"), 0755)
	_ = os.MkdirAll(filepath.Join(skillDir, ".git"), 0755)

	// Write SKILL.md
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: k8s-deploy
description: Kubernetes automated rolling deployments
---
# K8s Deploy Runbook
Follow steps to deploy.`), 0644)

	// Write helper scripts & references
	_ = os.WriteFile(filepath.Join(skillDir, "scripts", "check_cluster.sh"), []byte(`#!/bin/bash\nkubectl cluster-info`), 0755)
	_ = os.WriteFile(filepath.Join(skillDir, "scripts", "deploy.py"), []byte(`print("deploying")`), 0755)
	_ = os.WriteFile(filepath.Join(skillDir, "references", "schema.json"), []byte(`{"type":"object"}`), 0644)
	_ = os.WriteFile(filepath.Join(skillDir, "templates", "service.yaml"), []byte(`apiVersion: v1\nkind: Service`), 0644)
	// Write ignored hidden file
	_ = os.WriteFile(filepath.Join(skillDir, ".git", "config"), []byte(`git config`), 0644)

	// Reload skills
	if err := loader.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	skill, ok := loader.Get("k8s-deploy")
	if !ok {
		t.Fatalf("expected k8s-deploy skill to be found")
	}

	if len(skill.Files) != 4 {
		t.Fatalf("expected 4 discovered companion files, got %d: %+v", len(skill.Files), skill.Files)
	}

	// Verify categories
	fileCategoryMap := make(map[string]string)
	for _, f := range skill.Files {
		fileCategoryMap[f.Path] = f.Category
	}

	if fileCategoryMap["scripts/check_cluster.sh"] != "script" {
		t.Errorf("expected script category for check_cluster.sh, got %s", fileCategoryMap["scripts/check_cluster.sh"])
	}
	if fileCategoryMap["scripts/deploy.py"] != "script" {
		t.Errorf("expected script category for deploy.py, got %s", fileCategoryMap["scripts/deploy.py"])
	}
	if fileCategoryMap["references/schema.json"] != "reference" {
		t.Errorf("expected reference category for schema.json, got %s", fileCategoryMap["references/schema.json"])
	}
	if fileCategoryMap["templates/service.yaml"] != "template" {
		t.Errorf("expected template category for service.yaml, got %s", fileCategoryMap["templates/service.yaml"])
	}
	if _, exists := fileCategoryMap[".git/config"]; exists {
		t.Errorf("hidden file .git/config should have been excluded")
	}

	// Test read_skill tool output includes manifest and directory
	reg := tools.NewRegistry()
	loader.RegisterSkillTools(reg)

	argsJSON, _ := json.Marshal(map[string]string{"name": "k8s-deploy"})
	res, err := reg.Execute(context.Background(), "read_skill", argsJSON)
	if err != nil {
		t.Fatalf("read_skill failed: %v", err)
	}

	resMap, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", res)
	}

	if resMap["directory"] != skillDir {
		t.Errorf("expected directory '%s', got '%v'", skillDir, resMap["directory"])
	}

	availFiles, ok := resMap["available_files"].([]SkillFile)
	if !ok || len(availFiles) != 4 {
		t.Errorf("expected available_files slice with 4 items, got: %+v", resMap["available_files"])
	}
}

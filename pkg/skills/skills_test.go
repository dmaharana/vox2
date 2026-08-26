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

func TestSkillsLoaderAndAgentSkillsStandard(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "skills-spec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create skill 1 in subfolder with full Agent Skills frontmatter
	skill1Dir := filepath.Join(tempDir, "code-review")
	_ = os.MkdirAll(skill1Dir, 0755)
	skill1Content := `---
name: code-review
description: Expert Go code reviewer
license: Apache-2.0
compatibility: Requires Go 1.22+
allowed-tools: read_file execute_script
metadata:
  version: 1.0.0
  author: ant-team
---
Detailed guidelines: Review code for idiomatic Go conventions, error handling, and performance.`
	_ = os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(skill1Content), 0644)

	// Create skill 2 with disable-model-invocation: true
	skill2Dir := filepath.Join(tempDir, "secret-workflow")
	_ = os.MkdirAll(skill2Dir, 0755)
	skill2Content := `---
name: secret-workflow
description: Internal admin deployment runbook
disable-model-invocation: true
---
Admin deployment procedures.`
	_ = os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte(skill2Content), 0644)

	// Create skill 3 as direct root .md file with valid frontmatter
	skill3Content := `---
name: weather-lookup
description: Helps with weather forecasting and radar alerts.
---
# Weather Skill instructions`
	_ = os.WriteFile(filepath.Join(tempDir, "weather-lookup.md"), []byte(skill3Content), 0644)

	// Create a non-skill markdown doc (e.g. README.md without frontmatter) that should be silently ignored
	_ = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Project Readme\nThis is just a doc."), 0644)

	loader := NewLoader(tempDir)
	if err := loader.Load(); err != nil {
		t.Fatalf("failed to load skills: %v", err)
	}

	skillsList := loader.List()
	if len(skillsList) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skillsList))
	}

	// Verify code-review skill
	s1, ok := loader.Get("code-review")
	if !ok || s1.Description != "Expert Go code reviewer" {
		t.Errorf("skill 1 parse mismatch: %+v", s1)
	}
	if s1.License != "Apache-2.0" || s1.Compatibility != "Requires Go 1.22+" || s1.AllowedTools != "read_file execute_script" {
		t.Errorf("metadata mismatch: license=%s, compat=%s, allowed=%s", s1.License, s1.Compatibility, s1.AllowedTools)
	}
	if s1.Metadata["version"] != "1.0.0" || s1.Metadata["author"] != "ant-team" {
		t.Errorf("metadata map mismatch: %+v", s1.Metadata)
	}

	// Verify secret-workflow has DisableModelInvocation = true
	s2, ok := loader.Get("secret-workflow")
	if !ok || !s2.DisableModelInvocation {
		t.Errorf("expected secret-workflow to have DisableModelInvocation=true")
	}

	// Verify XML format in BuildPromptSection
	promptSection := loader.BuildPromptSection()
	if !strings.Contains(promptSection, "<available_skills>") || !strings.Contains(promptSection, "</available_skills>") {
		t.Errorf("expected Agent Skills XML format in prompt section: %s", promptSection)
	}
	if !strings.Contains(promptSection, "<name>code-review</name>") {
		t.Errorf("expected code-review in XML prompt: %s", promptSection)
	}
	if !strings.Contains(promptSection, "<name>weather-lookup</name>") {
		t.Errorf("expected weather-lookup in XML prompt: %s", promptSection)
	}

	// Disable-model-invocation skill MUST be excluded from system prompt XML
	if strings.Contains(promptSection, "secret-workflow") {
		t.Errorf("disabled-model-invocation skill should NOT be present in prompt section: %s", promptSection)
	}

	// Progressive disclosure check: full body should NOT be present in prompt section
	if strings.Contains(promptSection, "Detailed guidelines:") {
		t.Errorf("progressive disclosure violation: full content found in prompt section")
	}

	// Register tools and test read_skill
	reg := tools.NewRegistry()
	loader.RegisterSkillTools(reg)

	argsJSON, _ := json.Marshal(map[string]string{"name": "code-review"})
	res, err := reg.Execute(context.Background(), "read_skill", argsJSON)
	if err != nil {
		t.Fatalf("unexpected error reading skill: %v", err)
	}
	resMap, ok := res.(map[string]any)
	if !ok || !strings.Contains(resMap["content"].(string), "Detailed guidelines:") {
		t.Errorf("expected skill content in result: %+v", res)
	}
	if resMap["license"] != "Apache-2.0" {
		t.Errorf("expected license in read_skill response")
	}

	// Secret workflow can still be read explicitly
	args2JSON, _ := json.Marshal(map[string]string{"name": "secret-workflow"})
	res2, err := reg.Execute(context.Background(), "read_skill", args2JSON)
	if err != nil {
		t.Fatalf("unexpected error reading secret skill: %v", err)
	}
	resMap2, ok := res2.(map[string]any)
	if !ok || resMap2["disable_model_invocation"] != true {
		t.Errorf("expected disable_model_invocation: true in result")
	}
}

func TestSkillsCollisionAndValidationDiagnostics(t *testing.T) {
	tempDir1, err := os.MkdirTemp("", "skills-dir1-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "skills-dir2-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	// Create skill in dir1
	_ = os.MkdirAll(filepath.Join(tempDir1, "pdf-tools"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir1, "pdf-tools", "SKILL.md"), []byte(`---
name: pdf-tools
description: PDF tools from project dir1
---
Dir1 PDF instructions.`), 0644)

	// Create skill with same name in dir2 (collision)
	_ = os.MkdirAll(filepath.Join(tempDir2, "pdf-tools"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir2, "pdf-tools", "SKILL.md"), []byte(`---
name: pdf-tools
description: PDF tools from global dir2
---
Dir2 PDF instructions.`), 0644)

	// Create skill with invalid name warning (uppercase, consecutive hyphens)
	_ = os.MkdirAll(filepath.Join(tempDir1, "invalid-skill"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir1, "invalid-skill", "SKILL.md"), []byte(`---
name: Invalid--Name
description: Skill with name issues
---
Content.`), 0644)

	loader := NewLoader(tempDir1)
	loader.AddDirectory(tempDir2, "user")
	if err := loader.Load(); err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Verify collision diagnostic recorded
	diags := loader.Diagnostics()
	var collisionFound, nameWarningFound bool
	for _, d := range diags {
		if d.Type == "collision" && d.SkillName == "pdf-tools" {
			collisionFound = true
		}
		if d.Type == "warning" && strings.Contains(d.Message, "invalid characters") {
			nameWarningFound = true
		}
	}

	if !collisionFound {
		t.Errorf("expected collision diagnostic for pdf-tools")
	}
	if !nameWarningFound {
		t.Errorf("expected name warning diagnostic for Invalid--Name")
	}

	// Verify the first directory won the collision
	s, ok := loader.Get("pdf-tools")
	if !ok || s.Description != "PDF tools from project dir1" {
		t.Errorf("expected dir1 skill to win collision: %+v", s)
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

	// Create a complete multi-file skill folder
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

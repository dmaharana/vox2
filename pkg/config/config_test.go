package config

import (
	"os"
	"testing"
)

func TestConfigLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log_level info, got %s", cfg.LogLevel)
	}
	if cfg.SkillsDir != "./skills" {
		t.Errorf("expected default skills_dir ./skills, got %s", cfg.SkillsDir)
	}
}

func TestConfigEnvOverride(t *testing.T) {
	_ = os.Setenv("PORT", "9090")
	_ = os.Setenv("LOG_LEVEL", "debug")
	_ = os.Setenv("LLM_MODEL", "gpt-4o-mini")
	defer func() {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Unsetenv("LLM_MODEL")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level debug, got %s", cfg.LogLevel)
	}
	if cfg.LLMModel != "gpt-4o-mini" {
		t.Errorf("expected model gpt-4o-mini, got %s", cfg.LLMModel)
	}
}

func TestConfigUpdate(t *testing.T) {
	cfg, _ := Load()
	cfg.Update(Config{
		LLMModel:  "claude-3-5-sonnet",
		LLMAPIKey: "secret-key",
	})

	clone := cfg.Clone()
	if clone.LLMModel != "claude-3-5-sonnet" {
		t.Errorf("expected updated model claude-3-5-sonnet, got %s", clone.LLMModel)
	}
	if clone.LLMAPIKey != "secret-key" {
		t.Errorf("expected updated api key secret-key, got %s", clone.LLMAPIKey)
	}
}

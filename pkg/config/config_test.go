package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-harness/pkg/crypto"
	"go-harness/pkg/db"
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
	if cfg.LLMAuthType != "api_key" {
		t.Errorf("expected default llm_auth_type api_key, got %s", cfg.LLMAuthType)
	}
}

func TestConfigEnvOverride(t *testing.T) {
	_ = os.Setenv("PORT", "9090")
	_ = os.Setenv("LOG_LEVEL", "debug")
	_ = os.Setenv("LLM_MODEL", "gpt-4o-mini")
	_ = os.Setenv("LLM_AUTH_TYPE", "oauth2")
	_ = os.Setenv("LLM_OAUTH_CLIENT_ID", "test-client-id")
	_ = os.Setenv("LLM_OAUTH_CLIENT_SECRET", "test-client-secret")
	defer func() {
		_ = os.Unsetenv("PORT")
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Unsetenv("LLM_MODEL")
		_ = os.Unsetenv("LLM_AUTH_TYPE")
		_ = os.Unsetenv("LLM_OAUTH_CLIENT_ID")
		_ = os.Unsetenv("LLM_OAUTH_CLIENT_SECRET")
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
	if cfg.LLMAuthType != "oauth2" {
		t.Errorf("expected auth type oauth2, got %s", cfg.LLMAuthType)
	}
	if cfg.LLMOAuthClientID != "test-client-id" {
		t.Errorf("expected client id test-client-id, got %s", cfg.LLMOAuthClientID)
	}
}

func TestConfigUpdate(t *testing.T) {
	cfg, _ := Load()
	cfg.Update(Config{
		LLMModel:             "claude-3-5-sonnet",
		LLMAPIKey:            "secret-key",
		LLMAuthType:          "oauth2",
		LLMOAuthClientID:     "cid-123",
		LLMOAuthClientSecret: "csec-456",
		LLMOAuthTokenURL:     "https://oauth2.googleapis.com/token",
		LLMOAuthScopes:       "https://www.googleapis.com/auth/cloud-platform",
	})

	clone := cfg.Clone()
	if clone.LLMModel != "claude-3-5-sonnet" {
		t.Errorf("expected updated model claude-3-5-sonnet, got %s", clone.LLMModel)
	}
	if clone.LLMAPIKey != "secret-key" {
		t.Errorf("expected updated api key secret-key, got %s", clone.LLMAPIKey)
	}
	if clone.LLMAuthType != "oauth2" {
		t.Errorf("expected updated auth type oauth2, got %s", clone.LLMAuthType)
	}
	if clone.LLMOAuthClientID != "cid-123" {
		t.Errorf("expected cid-123, got %s", clone.LLMOAuthClientID)
	}
	if clone.LLMOAuthClientSecret != "csec-456" {
		t.Errorf("expected csec-456, got %s", clone.LLMOAuthClientSecret)
	}
}

func TestConfigDBSaveAndLoadEncryption(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cfg-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "config_test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	cfg, _ := Load()
	cfg.LLMAPIKey = "sk-test-super-secret-api-key-999"
	cfg.LLMAuthType = "oauth2"
	cfg.LLMOAuthClientID = "oauth-client-abc"
	cfg.LLMOAuthClientSecret = "oauth-client-secret-xyz"
	cfg.LLMOAuthTokenURL = "https://oauth2.googleapis.com/token"
	cfg.LLMOAuthScopes = "https://www.googleapis.com/auth/generative-language"

	// Save to DB
	if err := cfg.SaveToDB(database); err != nil {
		t.Fatalf("SaveToDB failed: %v", err)
	}

	// Verify directly in DB that secrets are ENCRYPTED and not stored in plaintext!
	rawKey, err := database.GetSetting("llm_api_key")
	if err != nil {
		t.Fatalf("GetSetting rawKey failed: %v", err)
	}
	if !strings.HasPrefix(rawKey, crypto.Prefix) {
		t.Fatalf("Expected raw API key in DB to have '%s' prefix, got '%s'", crypto.Prefix, rawKey)
	}
	if rawKey == "sk-test-super-secret-api-key-999" {
		t.Fatalf("SECURITY VIOLATION: API key is stored in plain text in database!")
	}

	rawSecret, err := database.GetSetting("llm_oauth_client_secret")
	if err != nil {
		t.Fatalf("GetSetting rawSecret failed: %v", err)
	}
	if !strings.HasPrefix(rawSecret, crypto.Prefix) {
		t.Fatalf("Expected raw client secret in DB to have '%s' prefix, got '%s'", crypto.Prefix, rawSecret)
	}
	if rawSecret == "oauth-client-secret-xyz" {
		t.Fatalf("SECURITY VIOLATION: OAuth client secret is stored in plain text in database!")
	}

	// Now load into a new config instance and verify decryption works seamlessly
	cfgLoaded, _ := Load()
	if err := cfgLoaded.LoadFromDB(database); err != nil {
		t.Fatalf("LoadFromDB failed: %v", err)
	}

	if cfgLoaded.LLMAPIKey != "sk-test-super-secret-api-key-999" {
		t.Errorf("Expected decrypted API key 'sk-test-super-secret-api-key-999', got '%s'", cfgLoaded.LLMAPIKey)
	}
	if cfgLoaded.LLMOAuthClientSecret != "oauth-client-secret-xyz" {
		t.Errorf("Expected decrypted client secret 'oauth-client-secret-xyz', got '%s'", cfgLoaded.LLMOAuthClientSecret)
	}
	if cfgLoaded.LLMAuthType != "oauth2" {
		t.Errorf("Expected auth type 'oauth2', got '%s'", cfgLoaded.LLMAuthType)
	}
	if cfgLoaded.LLMOAuthClientID != "oauth-client-abc" {
		t.Errorf("Expected client ID 'oauth-client-abc', got '%s'", cfgLoaded.LLMOAuthClientID)
	}
	if cfgLoaded.LLMOAuthTokenURL != "https://oauth2.googleapis.com/token" {
		t.Errorf("Expected token URL 'https://oauth2.googleapis.com/token', got '%s'", cfgLoaded.LLMOAuthTokenURL)
	}
}

func TestConfigCopilotFlags(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	args := []string{
		"--provider", "copilot",
		"--model", "gpt-5-mini",
		"--port", "8888",
		"--host", "127.0.0.1",
		"--log-level", "warn",
		"--copilot-binary", "/custom/bin/copilot",
		"--copilot-timeout", "90",
	}

	if err := cfg.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}

	if cfg.LLMProvider != "copilot" {
		t.Errorf("expected provider copilot, got %s", cfg.LLMProvider)
	}
	if cfg.LLMModel != "gpt-5-mini" {
		t.Errorf("expected model gpt-5-mini, got %s", cfg.LLMModel)
	}
	if cfg.Port != 8888 {
		t.Errorf("expected port 8888, got %d", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Host)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("expected log_level warn, got %s", cfg.LogLevel)
	}
	if cfg.CopilotBinary != "/custom/bin/copilot" {
		t.Errorf("expected copilot binary /custom/bin/copilot, got %s", cfg.CopilotBinary)
	}
	if cfg.CopilotTimeout != 90 {
		t.Errorf("expected copilot timeout 90, got %d", cfg.CopilotTimeout)
	}
}

func TestConfigCopilotDBSaveAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cfg-copilot-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "copilot_cfg_test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	cfg, _ := Load()
	cfg.LLMProvider = "copilot"
	cfg.LLMModel = "claude-3.5-sonnet"
	cfg.CopilotBinary = "/usr/local/bin/copilot"
	cfg.CopilotTimeout = 180

	if err := cfg.SaveToDB(database); err != nil {
		t.Fatalf("SaveToDB failed: %v", err)
	}

	cfgLoaded, _ := Load()
	if err := cfgLoaded.LoadFromDB(database); err != nil {
		t.Fatalf("LoadFromDB failed: %v", err)
	}

	if cfgLoaded.LLMProvider != "copilot" {
		t.Errorf("expected LLMProvider 'copilot', got '%s'", cfgLoaded.LLMProvider)
	}
	if cfgLoaded.LLMModel != "claude-3.5-sonnet" {
		t.Errorf("expected LLMModel 'claude-3.5-sonnet', got '%s'", cfgLoaded.LLMModel)
	}
	if cfgLoaded.CopilotBinary != "/usr/local/bin/copilot" {
		t.Errorf("expected CopilotBinary '/usr/local/bin/copilot', got '%s'", cfgLoaded.CopilotBinary)
	}
	if cfgLoaded.CopilotTimeout != 180 {
		t.Errorf("expected CopilotTimeout 180, got %d", cfgLoaded.CopilotTimeout)
	}
}


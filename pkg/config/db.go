package config

import (
	"fmt"
	"strconv"

	"go-harness/pkg/crypto"
	"go-harness/pkg/db"
)

// LoadFromDB overrides config values with any persisted settings in SQLite.
func (c *Config) LoadFromDB(database *db.DB) error {
	if database == nil {
		return nil
	}
	settings, err := database.GetAllSettings()
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := settings["llm_provider"]; ok && v != "" {
		c.LLMProvider = v
	}
	if v, ok := settings["llm_base_url"]; ok && v != "" {
		c.LLMBaseURL = v
	}
	if v, ok := settings["llm_model"]; ok && v != "" {
		c.LLMModel = v
	}
	if v, ok := settings["llm_api_key"]; ok && v != "" {
		decrypted, err := crypto.Decrypt(v)
		if err == nil {
			c.LLMAPIKey = decrypted
		} else {
			c.LLMAPIKey = v
		}
	}
	if v, ok := settings["llm_auth_type"]; ok && v != "" {
		c.LLMAuthType = v
	}
	if v, ok := settings["llm_oauth_client_id"]; ok && v != "" {
		c.LLMOAuthClientID = v
	}
	if v, ok := settings["llm_oauth_client_secret"]; ok && v != "" {
		decrypted, err := crypto.Decrypt(v)
		if err == nil {
			c.LLMOAuthClientSecret = decrypted
		} else {
			c.LLMOAuthClientSecret = v
		}
	}
	if v, ok := settings["llm_oauth_token_url"]; ok && v != "" {
		c.LLMOAuthTokenURL = v
	}
	if v, ok := settings["llm_oauth_scopes"]; ok && v != "" {
		c.LLMOAuthScopes = v
	}
	if v, ok := settings["llm_temperature"]; ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.LLMTemperature = f
		}
	}
	if v, ok := settings["llm_max_tokens"]; ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.LLMMaxTokens = i
		}
	}
	if v, ok := settings["copilot_binary"]; ok && v != "" {
		c.CopilotBinary = v
	}
	if v, ok := settings["copilot_timeout"]; ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.CopilotTimeout = i
		}
	}
	if v, ok := settings["log_level"]; ok && v != "" {
		c.LogLevel = v
	}
	if v, ok := settings["skills_dir"]; ok && v != "" {
		c.SkillsDir = v
	}

	return nil
}

// SaveToDB stores current LLM and runtime settings in SQLite.
func (c *Config) SaveToDB(database *db.DB) error {
	if database == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	_ = database.SetSetting("llm_provider", c.LLMProvider)
	_ = database.SetSetting("llm_base_url", c.LLMBaseURL)
	_ = database.SetSetting("llm_model", c.LLMModel)

	if c.LLMAPIKey != "" {
		encryptedKey, err := crypto.Encrypt(c.LLMAPIKey)
		if err == nil {
			_ = database.SetSetting("llm_api_key", encryptedKey)
		} else {
			_ = database.SetSetting("llm_api_key", c.LLMAPIKey)
		}
	}

	_ = database.SetSetting("llm_auth_type", c.LLMAuthType)
	_ = database.SetSetting("llm_oauth_client_id", c.LLMOAuthClientID)
	if c.LLMOAuthClientSecret != "" {
		encryptedSecret, err := crypto.Encrypt(c.LLMOAuthClientSecret)
		if err == nil {
			_ = database.SetSetting("llm_oauth_client_secret", encryptedSecret)
		} else {
			_ = database.SetSetting("llm_oauth_client_secret", c.LLMOAuthClientSecret)
		}
	}
	_ = database.SetSetting("llm_oauth_token_url", c.LLMOAuthTokenURL)
	_ = database.SetSetting("llm_oauth_scopes", c.LLMOAuthScopes)

	_ = database.SetSetting("llm_temperature", fmt.Sprintf("%f", c.LLMTemperature))
	_ = database.SetSetting("llm_max_tokens", fmt.Sprintf("%d", c.LLMMaxTokens))
	_ = database.SetSetting("copilot_binary", c.CopilotBinary)
	_ = database.SetSetting("copilot_timeout", fmt.Sprintf("%d", c.CopilotTimeout))
	_ = database.SetSetting("log_level", c.LogLevel)
	_ = database.SetSetting("skills_dir", c.SkillsDir)

	return nil
}

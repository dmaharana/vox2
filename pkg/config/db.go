package config

import (
	"fmt"
	"strconv"

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

	if v, ok := settings["llm_base_url"]; ok && v != "" {
		c.LLMBaseURL = v
	}
	if v, ok := settings["llm_model"]; ok && v != "" {
		c.LLMModel = v
	}
	if v, ok := settings["llm_api_key"]; ok && v != "" {
		c.LLMAPIKey = v
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

	_ = database.SetSetting("llm_base_url", c.LLMBaseURL)
	_ = database.SetSetting("llm_model", c.LLMModel)
	if c.LLMAPIKey != "" {
		_ = database.SetSetting("llm_api_key", c.LLMAPIKey)
	}
	_ = database.SetSetting("llm_temperature", fmt.Sprintf("%f", c.LLMTemperature))
	_ = database.SetSetting("llm_max_tokens", fmt.Sprintf("%d", c.LLMMaxTokens))
	_ = database.SetSetting("log_level", c.LogLevel)
	_ = database.SetSetting("skills_dir", c.SkillsDir)

	return nil
}

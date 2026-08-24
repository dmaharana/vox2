package config

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

// Config holds runtime configuration for the Go agent harness.
type Config struct {
	mu sync.RWMutex

	// Server
	Port int    `json:"port"`
	Host string `json:"host"`

	// Logging
	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"` // "console" or "json"

	// Paths
	SkillsDir string `json:"skills_dir"`
	DBPath    string `json:"db_path"`
	TraceFile string `json:"trace_file"`

	// OpenTelemetry
	OTelExporter string `json:"otel_exporter"` // "console", "file", "otlp", "all", "none"
	OTelEndpoint string `json:"otel_endpoint"`

	// LLM
	LLMBaseURL          string  `json:"llm_base_url"`
	LLMModel            string  `json:"llm_model"`
	LLMAPIKey           string  `json:"llm_api_key"`
	LLMAuthType         string  `json:"llm_auth_type"` // "api_key" or "oauth2"
	LLMOAuthClientID     string  `json:"llm_oauth_client_id"`
	LLMOAuthClientSecret string  `json:"llm_oauth_client_secret"`
	LLMOAuthTokenURL     string  `json:"llm_oauth_token_url"`
	LLMOAuthScopes       string  `json:"llm_oauth_scopes"`
	LLMTemperature      float64 `json:"llm_temperature"`
	LLMMaxTokens        int     `json:"llm_max_tokens"`
}

// Load loads configuration from optional .env file and environment variables with sensible defaults.
func Load(envPath ...string) (*Config, error) {
	if len(envPath) > 0 && envPath[0] != "" {
		_ = godotenv.Load(envPath[0])
	} else {
		_ = godotenv.Load(".env")
	}

	cfg := &Config{
		Port:                 getEnvInt("PORT", 8080),
		Host:                 getEnv("HOST", "0.0.0.0"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		LogFormat:            getEnv("LOG_FORMAT", "console"),
		SkillsDir:            getEnv("SKILLS_DIR", "./skills"),
		DBPath:               getEnv("DB_PATH", "./data/harness.db"),
		TraceFile:            getEnv("TRACE_FILE", "./logs/traces.json"),
		OTelExporter:         getEnv("OTEL_EXPORTER", "console"),
		OTelEndpoint:         getEnv("OTEL_ENDPOINT", "localhost:4317"),
		LLMBaseURL:           getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMModel:             getEnv("LLM_MODEL", "gpt-4o"),
		LLMAPIKey:            getEnv("LLM_API_KEY", ""),
		LLMAuthType:          getEnv("LLM_AUTH_TYPE", "api_key"),
		LLMOAuthClientID:     getEnv("LLM_OAUTH_CLIENT_ID", ""),
		LLMOAuthClientSecret: getEnv("LLM_OAUTH_CLIENT_SECRET", ""),
		LLMOAuthTokenURL:     getEnv("LLM_OAUTH_TOKEN_URL", ""),
		LLMOAuthScopes:       getEnv("LLM_OAUTH_SCOPES", ""),
		LLMTemperature:       getEnvFloat("LLM_TEMPERATURE", 0.7),
		LLMMaxTokens:         getEnvInt("LLM_MAX_TOKENS", 4096),
	}

	// Ensure directories for db and traces exist
	if dir := filepath.Dir(cfg.DBPath); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}
	if dir := filepath.Dir(cfg.TraceFile); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	return cfg, nil
}

// Clone returns a thread-safe copy of Config.
func (c *Config) Clone() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Config{
		Port:                 c.Port,
		Host:                 c.Host,
		LogLevel:             c.LogLevel,
		LogFormat:            c.LogFormat,
		SkillsDir:            c.SkillsDir,
		DBPath:               c.DBPath,
		TraceFile:            c.TraceFile,
		OTelExporter:         c.OTelExporter,
		OTelEndpoint:         c.OTelEndpoint,
		LLMBaseURL:           c.LLMBaseURL,
		LLMModel:             c.LLMModel,
		LLMAPIKey:            c.LLMAPIKey,
		LLMAuthType:          c.LLMAuthType,
		LLMOAuthClientID:     c.LLMOAuthClientID,
		LLMOAuthClientSecret: c.LLMOAuthClientSecret,
		LLMOAuthTokenURL:     c.LLMOAuthTokenURL,
		LLMOAuthScopes:       c.LLMOAuthScopes,
		LLMTemperature:       c.LLMTemperature,
		LLMMaxTokens:         c.LLMMaxTokens,
	}
}

// Update updates runtime mutable fields thread-safely.
func (c *Config) Update(newCfg Config) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if newCfg.LogLevel != "" {
		c.LogLevel = newCfg.LogLevel
	}
	if newCfg.SkillsDir != "" {
		c.SkillsDir = newCfg.SkillsDir
	}
	if newCfg.LLMBaseURL != "" {
		c.LLMBaseURL = newCfg.LLMBaseURL
	}
	if newCfg.LLMModel != "" {
		c.LLMModel = newCfg.LLMModel
	}
	if newCfg.LLMAPIKey != "" {
		c.LLMAPIKey = newCfg.LLMAPIKey
	}
	if newCfg.LLMAuthType != "" {
		c.LLMAuthType = newCfg.LLMAuthType
	}
	if newCfg.LLMOAuthClientID != "" {
		c.LLMOAuthClientID = newCfg.LLMOAuthClientID
	}
	if newCfg.LLMOAuthClientSecret != "" {
		c.LLMOAuthClientSecret = newCfg.LLMOAuthClientSecret
	}
	if newCfg.LLMOAuthTokenURL != "" {
		c.LLMOAuthTokenURL = newCfg.LLMOAuthTokenURL
	}
	if newCfg.LLMOAuthScopes != "" {
		c.LLMOAuthScopes = newCfg.LLMOAuthScopes
	}
	if newCfg.LLMTemperature >= 0 {
		c.LLMTemperature = newCfg.LLMTemperature
	}
	if newCfg.LLMMaxTokens > 0 {
		c.LLMMaxTokens = newCfg.LLMMaxTokens
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

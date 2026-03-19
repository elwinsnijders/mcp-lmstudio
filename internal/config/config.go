package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

type Config struct {
	APIBase        string
	APIToken       string
	Model          string
	ContextLength  int
	RequestTimeout int // minutes

	MaxSessionTokens       int
	TokenWarningThreshold  float64
	TokenCriticalThreshold float64

	ConfigFile   string
	SessionsDir  string
	ProgressDir  string
	ChatlogDir   string
	ArtifactsDir string
	LogFile      string

	App AppConfig
}

type AppConfig struct {
	SharedSystemPrompt string                      `json:"shared_system_prompt"`
	Profiles           map[string]ProfileConfig     `json:"profiles"`
	Integrations       map[string]IntegrationConfig `json:"integrations"`
}

type ProfileConfig struct {
	Label         string   `json:"label"`
	Description   string   `json:"description"`
	SystemPrompt  string   `json:"system_prompt"`
	Model         string   `json:"model,omitempty"`
	Temperature   float64  `json:"temperature,omitempty"`
	ContextLength int      `json:"context_length,omitempty"`
	Integrations  []string `json:"integrations"`

	TopP            float64 `json:"top_p,omitempty"`
	TopK            int     `json:"top_k,omitempty"`
	MinP            float64 `json:"min_p,omitempty"`
	RepeatPenalty   float64 `json:"repeat_penalty,omitempty"`
	MaxOutputTokens int     `json:"max_output_tokens,omitempty"`
	Reasoning       string  `json:"reasoning,omitempty"`
}

type IntegrationConfig struct {
	Label        string            `json:"label"`
	Description  string            `json:"description"`
	Type         string            `json:"type"` // "plugin" or "ephemeral_mcp"
	ID           string            `json:"id,omitempty"`
	ServerLabel  string            `json:"server_label,omitempty"`
	ServerURL    string            `json:"server_url,omitempty"`
	AllowedTools []string          `json:"allowed_tools,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

func Load() (*Config, error) {
	cfg := &Config{
		APIBase:                getEnv("LMSTUDIO_API_BASE", "http://127.0.0.1:1234"),
		APIToken:               os.Getenv("LMSTUDIO_API_TOKEN"),
		Model:                  getEnv("LMSTUDIO_MODEL", "default"),
		ContextLength:          getEnvInt("LMSTUDIO_CONTEXT_LENGTH", 8192),
		RequestTimeout:         getEnvInt("LMSTUDIO_REQUEST_TIMEOUT", 10),
		MaxSessionTokens:       getEnvInt("MAX_SESSION_TOKENS", 175000),
		TokenWarningThreshold:  getEnvFloat("TOKEN_WARNING_THRESHOLD", 0.80),
		TokenCriticalThreshold: getEnvFloat("TOKEN_CRITICAL_THRESHOLD", 0.95),
		ConfigFile:             getEnv("CONFIG_FILE", "config.json"),
		SessionsDir:            getEnv("SESSIONS_DIR", "sessions"),
		ProgressDir:            getEnv("PROGRESS_DIR", "progress"),
		ChatlogDir:             getEnv("CHATLOG_DIR", "chatlogs"),
		ArtifactsDir:           getEnv("ARTIFACTS_DIR", "artifacts"),
		LogFile:                getEnv("LOG_FILE", defaultLogFile()),
	}

	if err := cfg.loadAppConfig(); err != nil {
		log.Printf("Warning: could not load config file %s: %v", cfg.ConfigFile, err)
		cfg.App = AppConfig{
			Profiles:     make(map[string]ProfileConfig),
			Integrations: make(map[string]IntegrationConfig),
		}
	}

	os.MkdirAll(cfg.SessionsDir, 0755)
	os.MkdirAll(cfg.ProgressDir, 0755)
	os.MkdirAll(cfg.ChatlogDir, 0755)
	os.MkdirAll(cfg.ArtifactsDir, 0755)

	return cfg, nil
}

func (c *Config) loadAppConfig() error {
	path := c.ConfigFile
	if !filepath.IsAbs(path) {
		if execPath, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(execPath), path)
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, &c.App); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	return nil
}

func defaultLogFile() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "lmstudio_audit.log")
	}
	return "/tmp/lmstudio_audit.log"
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		log.Printf("Warning: invalid %s value %q, using default %d", key, v, defaultVal)
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
		log.Printf("Warning: invalid %s value %q, using default %.2f", key, v, defaultVal)
	}
	return defaultVal
}

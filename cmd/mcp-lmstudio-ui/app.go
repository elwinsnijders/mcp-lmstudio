package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/infinitimeless/lmstudio-mcp/internal/chatlog"
)

type App struct {
	ctx          context.Context
	chatWatcher  *ChatWatcher
	groupWatcher *GroupWatcher
	dataDir      string
	debugLog     *os.File
}

func (a *App) dbg(format string, args ...interface{}) {
	if a.debugLog == nil {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(a.debugLog, "%s [app] %s\n", ts, fmt.Sprintf(format, args...))
	a.debugLog.Sync()
}

// SessionDTO is the frontend-facing session representation.
type SessionDTO struct {
	ID               string   `json:"id"`
	Task             string   `json:"task"`
	Profile          string   `json:"profile"`
	Model            string   `json:"model"`
	Status           string   `json:"status"`
	TokensUsed       int      `json:"tokensUsed"`
	TokensMax        int      `json:"tokensMax"`
	TokensPercent    float64  `json:"tokensPercent"`
	Exchanges        int      `json:"exchanges"`
	IntegrationKeys  []string `json:"integrationKeys"`
	Project          string   `json:"project,omitempty"`
	GroupID          string   `json:"groupId,omitempty"`
	GroupStep        int      `json:"groupStep,omitempty"`
	CreatedAt        string   `json:"createdAt"`
	LastActiveAt     string   `json:"lastActiveAt"`
	HasChatLog       bool     `json:"hasChatLog"`
}

// GroupDTO is the frontend-facing task group representation.
type GroupDTO struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Status       string   `json:"status"`
	TotalSteps   int      `json:"totalSteps"`
	CurrentStep  int      `json:"currentStep"`
	Succeeded    int      `json:"succeeded"`
	Failed       int      `json:"failed"`
	SessionIDs   []string `json:"sessionIds"`
	ChainMode    string   `json:"chainMode,omitempty"`
	StoppedEarly bool     `json:"stoppedEarly,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

// ConfigDTO wraps the config.json structure for the frontend.
type ConfigDTO struct {
	SharedSystemPrompt string                       `json:"shared_system_prompt"`
	Profiles           map[string]ProfileDTO        `json:"profiles"`
	Integrations       map[string]IntegrationDTO    `json:"integrations"`
}

type ProfileDTO struct {
	Label           string   `json:"label"`
	Description     string   `json:"description"`
	SystemPrompt    string   `json:"system_prompt"`
	Model           string   `json:"model"`
	Temperature     float64  `json:"temperature"`
	ContextLength   int      `json:"context_length"`
	TopP            float64  `json:"top_p"`
	TopK            int      `json:"top_k"`
	MinP            float64  `json:"min_p"`
	RepeatPenalty   float64  `json:"repeat_penalty"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	Reasoning       string   `json:"reasoning"`
	Integrations    []string `json:"integrations"`
}

type IntegrationDTO struct {
	Label        string            `json:"label"`
	Description  string            `json:"description"`
	Type         string            `json:"type"`
	ID           string            `json:"id,omitempty"`
	ServerLabel  string            `json:"server_label,omitempty"`
	ServerURL    string            `json:"server_url,omitempty"`
	AllowedTools []string          `json:"allowed_tools,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Command      []string          `json:"command,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

type SettingsDTO struct {
	APIBase                string  `json:"apiBase"`
	APIToken               string  `json:"apiToken"`
	Model                  string  `json:"model"`
	ContextLength          int     `json:"contextLength"`
	RequestTimeout         int     `json:"requestTimeout"`
	MaxSessionTokens       int     `json:"maxSessionTokens"`
	TokenWarningThreshold  float64 `json:"tokenWarningThreshold"`
	TokenCriticalThreshold float64 `json:"tokenCriticalThreshold"`
	SessionsDir            string  `json:"sessionsDir"`
	ProgressDir            string  `json:"progressDir"`
	ChatlogDir             string  `json:"chatlogDir"`
	ConfigFile             string  `json:"configFile"`
	LogFile                string  `json:"logFile"`
}

func NewApp() *App {
	return &App{
		chatWatcher:  NewChatWatcher(),
		groupWatcher: NewGroupWatcher(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.dataDir = findDataDir()
	a.loadDotEnv()
	logPath := filepath.Join(a.dataDir, "ui-debug.log")
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		a.debugLog = f
	}
	a.dbg("STARTUP dataDir=%s", a.dataDir)
}

func (a *App) shutdown(ctx context.Context) {
	a.dbg("SHUTDOWN")
	a.chatWatcher.Stop()
	a.groupWatcher.Stop()
	if a.debugLog != nil {
		a.debugLog.Close()
	}
}

func findDataDir() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
			return dir
		}
		for i := 0; i < 3; i++ {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
			if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
				return dir
			}
		}
	}

	if wd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(wd, "config.json")); err == nil {
			return wd
		}
	}

	return "."
}

func (a *App) GetDataDir() string {
	return a.dataDir
}

func (a *App) SetDataDir(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("directory not accessible: %w", err)
	}
	a.dataDir = dir
	return nil
}

// ── Sessions ────────────────────────────────────────────────────────────

type rawSession struct {
	ID               string    `json:"id"`
	Task             string    `json:"task"`
	Profile          string    `json:"profile,omitempty"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	TokensUsed       int       `json:"tokens_used"`
	TokensMax        int       `json:"tokens_max"`
	ResponseIDs      []string  `json:"response_ids"`
	IntegrationKeys  []string  `json:"integration_keys,omitempty"`
	Project          string    `json:"project,omitempty"`
	GroupID          string    `json:"group_id,omitempty"`
	GroupStep        int       `json:"group_step,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	LastActiveAt     time.Time `json:"last_active_at"`
}

func (a *App) ListSessions() ([]SessionDTO, error) {
	sessionsPath := filepath.Join(a.dataDir, "sessions", "sessions.json")
	data, err := os.ReadFile(sessionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionDTO{}, nil
		}
		return nil, fmt.Errorf("reading sessions: %w", err)
	}

	var raw []rawSession
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing sessions: %w", err)
	}

	chatlogDir := filepath.Join(a.dataDir, "chatlogs")
	dtos := make([]SessionDTO, 0, len(raw))
	for _, s := range raw {
		pct := 0.0
		if s.TokensMax > 0 {
			pct = float64(s.TokensUsed) / float64(s.TokensMax) * 100
		}

		hasChatLog := false
		logPath := filepath.Join(chatlogDir, s.ID+".jsonl")
		if _, err := os.Stat(logPath); err == nil {
			hasChatLog = true
		}

		dtos = append(dtos, SessionDTO{
			ID:              s.ID,
			Task:            s.Task,
			Profile:         s.Profile,
			Model:           s.Model,
			Status:          s.Status,
			TokensUsed:      s.TokensUsed,
			TokensMax:       s.TokensMax,
			TokensPercent:   pct,
			Exchanges:       len(s.ResponseIDs),
			IntegrationKeys: s.IntegrationKeys,
			Project:         s.Project,
			GroupID:         s.GroupID,
			GroupStep:       s.GroupStep,
			CreatedAt:       s.CreatedAt.Format(time.RFC3339),
			LastActiveAt:    s.LastActiveAt.Format(time.RFC3339),
			HasChatLog:      hasChatLog,
		})
	}

	sort.Slice(dtos, func(i, j int) bool {
		return dtos[i].LastActiveAt > dtos[j].LastActiveAt
	})

	return dtos, nil
}

func (a *App) GetActiveSessions() ([]string, error) {
	sessions, err := a.ListSessions()
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, s := range sessions {
		if s.Status == "active" {
			ids = append(ids, s.ID)
		}
	}
	return ids, nil
}

// ── Task Groups ─────────────────────────────────────────────────────────

type rawGroup struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	TotalSteps   int       `json:"total_steps"`
	CurrentStep  int       `json:"current_step"`
	Succeeded    int       `json:"succeeded"`
	Failed       int       `json:"failed"`
	SessionIDs   []string  `json:"session_ids"`
	ChainMode    string    `json:"chain_mode,omitempty"`
	StoppedEarly bool      `json:"stopped_early,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (a *App) ListGroups() ([]GroupDTO, error) {
	groupsPath := filepath.Join(a.dataDir, "sessions", "groups.json")
	data, err := os.ReadFile(groupsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []GroupDTO{}, nil
		}
		return nil, fmt.Errorf("reading groups: %w", err)
	}

	var raw []rawGroup
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing groups: %w", err)
	}

	dtos := make([]GroupDTO, 0, len(raw))
	for _, g := range raw {
		dtos = append(dtos, rawGroupToDTO(&g))
	}

	sort.Slice(dtos, func(i, j int) bool {
		return dtos[i].UpdatedAt > dtos[j].UpdatedAt
	})

	return dtos, nil
}

func (a *App) GetGroup(id string) (*GroupDTO, error) {
	groups, err := a.ListGroups()
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		if g.ID == id {
			return &g, nil
		}
	}
	return nil, fmt.Errorf("group %q not found", id)
}

// ── Chat Logs ───────────────────────────────────────────────────────────

func (a *App) LoadChatLog(sessionID string) ([]chatlog.ChatEvent, error) {
	chatlogDir := filepath.Join(a.dataDir, "chatlogs")
	// Tail read keeps Wails main thread responsive on large logs (Live View / archive).
	return chatlog.ReadRecent(chatlogDir, sessionID, chatlog.DefaultRecentMaxBytes, chatlog.DefaultRecentMaxEvents)
}

func (a *App) StartChatWatch(sessionID string) {
	a.dbg("StartChatWatch session=%s", sessionID)
	chatlogDir := filepath.Join(a.dataDir, "chatlogs")
	path := filepath.Join(chatlogDir, sessionID+".jsonl")
	a.chatWatcher.Start(a.ctx, path, sessionID)
}

func (a *App) StopChatWatch() {
	a.dbg("StopChatWatch")
	a.chatWatcher.Stop()
}

func (a *App) StartGroupWatch(groupID string, initialLen int) {
	a.dbg("StartGroupWatch group=%s initialLen=%d", groupID, initialLen)
	a.chatWatcher.Stop()
	a.groupWatcher.Start(a.ctx, a.dataDir, groupID, initialLen)
}

func (a *App) StopGroupWatch() {
	a.dbg("StopGroupWatch")
	a.groupWatcher.Stop()
}

// ── Config (profiles & integrations) ────────────────────────────────────

func (a *App) configPath() string {
	return filepath.Join(a.dataDir, "config.json")
}

func (a *App) LoadConfig() (*ConfigDTO, error) {
	data, err := os.ReadFile(a.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			empty := &ConfigDTO{
				Profiles:     make(map[string]ProfileDTO),
				Integrations: make(map[string]IntegrationDTO),
			}
			return empty, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var dto ConfigDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if dto.Profiles == nil {
		dto.Profiles = make(map[string]ProfileDTO)
	}
	if dto.Integrations == nil {
		dto.Integrations = make(map[string]IntegrationDTO)
	}
	return &dto, nil
}

func (a *App) SaveConfig(dto ConfigDTO) error {
	data, err := json.MarshalIndent(dto, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(a.configPath(), data, 0644)
}

// ── Settings (env-based config) ─────────────────────────────────────────

func (a *App) LoadSettings() SettingsDTO {
	return SettingsDTO{
		APIBase:                envOrDefault("LMSTUDIO_API_BASE", "http://127.0.0.1:1234"),
		APIToken:               os.Getenv("LMSTUDIO_API_TOKEN"),
		Model:                  envOrDefault("LMSTUDIO_MODEL", "default"),
		ContextLength:          envIntOrDefault("LMSTUDIO_CONTEXT_LENGTH", 8192),
		RequestTimeout:         envIntOrDefault("LMSTUDIO_REQUEST_TIMEOUT", 10),
		MaxSessionTokens:       envIntOrDefault("MAX_SESSION_TOKENS", 175000),
		TokenWarningThreshold:  envFloatOrDefault("TOKEN_WARNING_THRESHOLD", 0.80),
		TokenCriticalThreshold: envFloatOrDefault("TOKEN_CRITICAL_THRESHOLD", 0.95),
		SessionsDir:            envOrDefault("SESSIONS_DIR", "sessions"),
		ProgressDir:            envOrDefault("PROGRESS_DIR", "progress"),
		ChatlogDir:             envOrDefault("CHATLOG_DIR", "chatlogs"),
		ConfigFile:             envOrDefault("CONFIG_FILE", "config.json"),
		LogFile:                envOrDefault("LOG_FILE", ""),
	}
}

func (a *App) SaveSettings(dto SettingsDTO) error {
	envPath := filepath.Join(a.dataDir, ".env")
	content := fmt.Sprintf(`LMSTUDIO_API_BASE=%s
LMSTUDIO_API_TOKEN=%s
LMSTUDIO_MODEL=%s
LMSTUDIO_CONTEXT_LENGTH=%d
LMSTUDIO_REQUEST_TIMEOUT=%d
MAX_SESSION_TOKENS=%d
TOKEN_WARNING_THRESHOLD=%.2f
TOKEN_CRITICAL_THRESHOLD=%.2f
SESSIONS_DIR=%s
PROGRESS_DIR=%s
CHATLOG_DIR=%s
CONFIG_FILE=%s
LOG_FILE=%s
`,
		dto.APIBase, dto.APIToken, dto.Model,
		dto.ContextLength, dto.RequestTimeout,
		dto.MaxSessionTokens,
		dto.TokenWarningThreshold, dto.TokenCriticalThreshold,
		dto.SessionsDir, dto.ProgressDir, dto.ChatlogDir,
		dto.ConfigFile, dto.LogFile,
	)
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		return err
	}
	a.loadDotEnv()
	return nil
}

// loadDotEnv reads the .env file beside config.json and sets each KEY=VALUE
// into the process environment so LoadSettings picks them up.
func (a *App) loadDotEnv() {
	envPath := filepath.Join(a.dataDir, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx > 0 {
			os.Setenv(line[:idx], line[idx+1:])
		}
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return def
}

func envFloatOrDefault(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return def
}

package progress

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Manager struct {
	dir string
}

func NewManager(dir string) *Manager {
	return &Manager{dir: dir}
}

type Info struct {
	SessionID    string
	Task         string
	Profile      string
	Model        string
	TokensUsed   int
	TokensMax    int
	StartedAt    time.Time
	LastActiveAt time.Time
	Summary      string
	Notes        string
}

func (m *Manager) Save(info *Info) (string, error) {
	filename := fmt.Sprintf("%s.md", info.SessionID)
	path := filepath.Join(m.dir, filename)

	taskTitle := info.Task
	if len(taskTitle) > 80 {
		taskTitle = taskTitle[:80] + "..."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Task Progress: %s\n\n", taskTitle)
	fmt.Fprintf(&b, "## Session Info\n")
	fmt.Fprintf(&b, "- Session ID: %s\n", info.SessionID)
	fmt.Fprintf(&b, "- Profile: %s\n", info.Profile)
	fmt.Fprintf(&b, "- Model: %s\n", info.Model)
	fmt.Fprintf(&b, "- Tokens Used: %d / %d\n", info.TokensUsed, info.TokensMax)
	fmt.Fprintf(&b, "- Started: %s\n", info.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- Last Active: %s\n\n", info.LastActiveAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "## Original Task\n%s\n\n", info.Task)
	fmt.Fprintf(&b, "## Progress Summary\n%s\n", info.Summary)

	if info.Notes != "" {
		fmt.Fprintf(&b, "\n## Orchestrator Notes\n%s\n", info.Notes)
	}

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return "", fmt.Errorf("writing progress file: %w", err)
	}

	return path, nil
}

func (m *Manager) Load(sessionID string) (string, error) {
	path := filepath.Join(m.dir, fmt.Sprintf("%s.md", sessionID))
	return m.LoadFile(path)
}

func (m *Manager) LoadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading progress file: %w", err)
	}
	return string(data), nil
}

func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing progress dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

const SaveProgressPrompt = `Summarize your progress on this task using exactly this format:

### Work Completed
[List what you accomplished]

### Errors Encountered & Solutions
[List any errors and how you resolved them, or "None"]

### Files Modified
[List files you created or modified with brief descriptions, or "None"]

### Remaining Work
[List what still needs to be done, or "Task complete"]

### Key Decisions
[List important technical decisions you made and why]`

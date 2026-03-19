package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	dir string
	mu  sync.Mutex
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating artifacts dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) Store(sessionID, tool string, args json.RawMessage, output string, providerInfo json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessDir := filepath.Join(s.dir, sessionID)
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		return fmt.Errorf("creating session artifacts dir: %w", err)
	}

	index, err := s.readIndexLocked(sessionID)
	if err != nil {
		return err
	}

	seq := len(index)
	art := Artifact{
		SessionID:    sessionID,
		Sequence:     seq,
		Tool:         tool,
		Arguments:    args,
		FilePath:     extractFilePath(args),
		ContentSize:  len(output),
		ProviderInfo: providerInfo,
		Timestamp:    time.Now(),
	}

	contentPath := filepath.Join(sessDir, fmt.Sprintf("%d.content", seq))
	if err := os.WriteFile(contentPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("writing artifact content: %w", err)
	}

	index = append(index, art)
	return s.writeIndexLocked(sessionID, index)
}

func (s *Store) List(sessionID string) ([]Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readIndexLocked(sessionID)
}

func (s *Store) Get(sessionID string, seq int) (*Artifact, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.readIndexLocked(sessionID)
	if err != nil {
		return nil, "", err
	}

	if seq < 0 || seq >= len(index) {
		return nil, "", fmt.Errorf("artifact %d not found in session %s", seq, sessionID)
	}

	content, err := s.readContentLocked(sessionID, seq)
	if err != nil {
		return nil, "", err
	}

	art := index[seq]
	return &art, content, nil
}

func (s *Store) GetByPath(sessionID, filePath string) (*Artifact, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.readIndexLocked(sessionID)
	if err != nil {
		return nil, "", err
	}

	for i := len(index) - 1; i >= 0; i-- {
		if index[i].FilePath == filePath {
			content, err := s.readContentLocked(sessionID, index[i].Sequence)
			if err != nil {
				return nil, "", err
			}
			art := index[i]
			return &art, content, nil
		}
	}

	return nil, "", fmt.Errorf("no artifact with path %q in session %s", filePath, sessionID)
}

func (s *Store) readIndexLocked(sessionID string) ([]Artifact, error) {
	path := filepath.Join(s.dir, sessionID, "index.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading artifact index: %w", err)
	}

	var index []Artifact
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parsing artifact index: %w", err)
	}
	return index, nil
}

func (s *Store) writeIndexLocked(sessionID string, index []Artifact) error {
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling artifact index: %w", err)
	}
	path := filepath.Join(s.dir, sessionID, "index.json")
	return os.WriteFile(path, data, 0644)
}

func (s *Store) readContentLocked(sessionID string, seq int) (string, error) {
	path := filepath.Join(s.dir, sessionID, fmt.Sprintf("%d.content", seq))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading artifact content: %w", err)
	}
	return string(data), nil
}

// extractFilePath tries to pull a file path from tool call arguments.
func extractFilePath(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	var m map[string]interface{}
	if json.Unmarshal(args, &m) != nil {
		return ""
	}
	for _, key := range []string{"path", "file_path", "filename", "file", "filePath"} {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

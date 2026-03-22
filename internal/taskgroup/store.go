package taskgroup

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	TypeQueue = "queue"
	TypeChain = "chain"
	TypeLoop  = "loop"

	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

type Group struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	TotalSteps   int       `json:"total_steps"`
	CurrentStep  int       `json:"current_step"`
	Succeeded    int       `json:"succeeded"`
	Failed       int       `json:"failed"`
	SessionIDs   []string  `json:"session_ids"`
	ChainMode    string    `json:"chain_mode,omitempty"`
	Directive    string    `json:"directive,omitempty"`
	StopPhrase   string    `json:"stop_phrase,omitempty"`
	StoppedEarly bool      `json:"stopped_early,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Store struct {
	path   string
	mu     sync.RWMutex
	groups map[string]*Group
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating taskgroup dir: %w", err)
	}
	path := filepath.Join(dir, "groups.json")
	s := &Store{
		path:   path,
		groups: make(map[string]*Group),
	}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("loading groups: %w", err)
	}
	return s, nil
}

func (s *Store) Create(groupType string, totalSteps int) (*Group, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	g := &Group{
		ID:         id,
		Type:       groupType,
		Status:     StatusRunning,
		TotalSteps: totalSteps,
		SessionIDs: []string{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.groups[id] = g
	if err := s.save(); err != nil {
		return nil, err
	}
	cp := *g
	return &cp, nil
}

func (s *Store) Get(id string) (*Group, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[id]
	if !ok {
		return nil, false
	}
	cp := *g
	return &cp, true
}

func (s *Store) Update(g *Group) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g.UpdatedAt = time.Now()
	cp := *g
	s.groups[g.ID] = &cp
	return s.save()
}

func (s *Store) List() []*Group {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Group, 0, len(s.groups))
	for _, g := range s.groups {
		cp := *g
		result = append(result, &cp)
	}
	return result
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var groups []*Group
	if err := json.Unmarshal(data, &groups); err != nil {
		return fmt.Errorf("parsing groups file: %w", err)
	}
	for _, g := range groups {
		s.groups[g.ID] = g
	}
	return nil
}

func (s *Store) save() error {
	groups := make([]*Group, 0, len(s.groups))
	for _, g := range s.groups {
		groups = append(groups, g)
	}
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling groups: %w", err)
	}
	return os.WriteFile(s.path, data, 0644)
}

func generateID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating group ID: %w", err)
	}
	return fmt.Sprintf("grp_%x", b), nil
}

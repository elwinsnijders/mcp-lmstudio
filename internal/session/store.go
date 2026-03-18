package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	path     string
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewStore(dir string) (*Store, error) {
	path := filepath.Join(dir, "sessions.json")
	s := &Store{
		path:     path,
		sessions: make(map[string]*Session),
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("loading sessions: %w", err)
	}

	return s, nil
}

func (s *Store) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, false
	}

	cp := *sess
	return &cp, true
}

func (s *Store) Put(sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := *sess
	s.sessions[sess.ID] = &cp
	return s.save()
}

func (s *Store) List() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		cp := *sess
		result = append(result, &cp)
	}
	return result
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, id)
	return s.save()
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var sessions []*Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return fmt.Errorf("parsing sessions file: %w", err)
	}

	for _, sess := range sessions {
		s.sessions[sess.ID] = sess
	}
	return nil
}

func (s *Store) save() error {
	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling sessions: %w", err)
	}

	return os.WriteFile(s.path, data, 0644)
}

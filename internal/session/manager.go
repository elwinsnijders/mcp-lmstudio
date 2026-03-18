package session

import (
	"crypto/rand"
	"fmt"
	"time"
)

type Manager struct {
	store             *Store
	defaultMaxTokens  int
	warningThreshold  float64
	criticalThreshold float64
}

func NewManager(store *Store, defaultMaxTokens int, warningThreshold, criticalThreshold float64) *Manager {
	return &Manager{
		store:             store,
		defaultMaxTokens:  defaultMaxTokens,
		warningThreshold:  warningThreshold,
		criticalThreshold: criticalThreshold,
	}
}

func (m *Manager) Create(task, profile, model string, maxTokens int, integrationKeys []string) (*Session, error) {
	if maxTokens <= 0 {
		maxTokens = m.defaultMaxTokens
	}

	id, err := generateID()
	if err != nil {
		return nil, err
	}

	sess := &Session{
		ID:              id,
		Task:            task,
		Profile:         profile,
		Model:           model,
		Status:          StatusActive,
		TokensUsed:      0,
		TokensMax:       maxTokens,
		ResponseIDs:     []string{},
		IntegrationKeys: integrationKeys,
		CreatedAt:       time.Now(),
		LastActiveAt:    time.Now(),
	}

	if err := m.store.Put(sess); err != nil {
		return nil, fmt.Errorf("saving session: %w", err)
	}

	return sess, nil
}

func (m *Manager) Get(id string) (*Session, error) {
	sess, ok := m.store.Get(id)
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	return sess, nil
}

func (m *Manager) AddTokens(id string, inputTokens, outputTokens int, responseID string) (*Session, *TokenUsage, error) {
	sess, ok := m.store.Get(id)
	if !ok {
		return nil, nil, fmt.Errorf("session %q not found", id)
	}

	thisRequest := inputTokens + outputTokens
	sess.TokensUsed += thisRequest
	sess.LastActiveAt = time.Now()

	if responseID != "" {
		sess.ResponseIDs = append(sess.ResponseIDs, responseID)
		sess.LatestResponseID = responseID
	}

	usage := &TokenUsage{
		Used:        sess.TokensUsed,
		Max:         sess.TokensMax,
		Percentage:  float64(sess.TokensUsed) / float64(sess.TokensMax),
		ThisRequest: thisRequest,
	}

	if err := m.store.Put(sess); err != nil {
		return nil, nil, fmt.Errorf("saving session: %w", err)
	}

	return sess, usage, nil
}

func (m *Manager) UpdateStatus(id, status string) error {
	sess, ok := m.store.Get(id)
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}

	sess.Status = status
	sess.LastActiveAt = time.Now()
	return m.store.Put(sess)
}

func (m *Manager) List() []*Session {
	return m.store.List()
}

func (m *Manager) TokenWarning(usage *TokenUsage) string {
	if usage.Percentage >= m.criticalThreshold {
		return fmt.Sprintf("CRITICAL: Token usage at %.1f%%. Save progress immediately.", usage.Percentage*100)
	}
	if usage.Percentage >= m.warningThreshold {
		return fmt.Sprintf("WARNING: Token usage at %.1f%%. Consider saving progress.", usage.Percentage*100)
	}
	return ""
}

func (m *Manager) FormatTokenUsage(usage *TokenUsage) string {
	return fmt.Sprintf("[Tokens: %d / %d (%.1f%%) | This request: %d]",
		usage.Used, usage.Max, usage.Percentage*100, usage.ThisRequest)
}

func generateID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating session ID: %w", err)
	}
	return fmt.Sprintf("sess_%x", b), nil
}

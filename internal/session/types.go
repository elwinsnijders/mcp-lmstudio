package session

import "time"

const (
	StatusActive    = "active"
	StatusCompleted = "completed"
	StatusPaused    = "paused"
	StatusTokenLimit = "token_limit"
)

type Session struct {
	ID               string    `json:"id"`
	Task             string    `json:"task"`
	Profile          string    `json:"profile,omitempty"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	TokensUsed       int       `json:"tokens_used"`
	TokensMax        int       `json:"tokens_max"`
	ResponseIDs      []string  `json:"response_ids"`
	LatestResponseID string    `json:"latest_response_id"`
	IntegrationKeys  []string  `json:"integration_keys,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	LastActiveAt     time.Time `json:"last_active_at"`
}

type TokenUsage struct {
	Used        int     `json:"used"`
	Max         int     `json:"max"`
	Percentage  float64 `json:"percentage"`
	ThisRequest int     `json:"this_request"`
}

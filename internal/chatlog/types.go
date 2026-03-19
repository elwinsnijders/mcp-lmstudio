package chatlog

import "time"

const (
	EventUserMessage = "user_message"
	EventAIDelta     = "ai_delta"
	EventAIComplete  = "ai_complete"
	EventToolUse     = "tool_use"
	EventError       = "error"
)

type ChatEvent struct {
	Type      string     `json:"type"`
	SessionID string     `json:"session_id"`
	Timestamp time.Time  `json:"ts"`
	Content   string     `json:"content,omitempty"`
	Stats     *ChatStats `json:"stats,omitempty"`
	Tool      string     `json:"tool,omitempty"`
	Arguments string     `json:"arguments,omitempty"`
}

type ChatStats struct {
	InputTokens    int     `json:"input_tokens"`
	OutputTokens   int     `json:"output_tokens"`
	TokensPerSec   float64 `json:"tokens_per_sec"`
	TimeToFirstSec float64 `json:"time_to_first_sec"`
	ResponseID     string  `json:"response_id,omitempty"`
}

package chatlog

import "time"

const (
	EventUserMessage    = "user_message"
	EventAIDelta        = "ai_delta"
	EventAIComplete     = "ai_complete"
	EventToolUse        = "tool_use"
	EventError          = "error"
	EventReasoningStart = "reasoning_start"
	EventReasoningDelta = "reasoning_delta"
	EventReasoningEnd   = "reasoning_end"
	EventToolCallStart  = "tool_call_start"
	EventToolCallResult = "tool_call_result"
	EventStatus         = "status"
	EventGroupStart     = "group_start"
	EventGroupStep      = "group_step"
	EventGroupComplete  = "group_complete"
)

type ChatEvent struct {
	Type      string     `json:"type"`
	SessionID string     `json:"session_id"`
	Timestamp time.Time  `json:"ts"`
	Content   string     `json:"content,omitempty"`
	Stats     *ChatStats `json:"stats,omitempty"`
	Tool      string     `json:"tool,omitempty"`
	Arguments string     `json:"arguments,omitempty"`
	Phase     string     `json:"phase,omitempty"`
	Progress  *float64   `json:"progress,omitempty"`
	Success   *bool      `json:"success,omitempty"`
	Output    string     `json:"output,omitempty"`
	Reason    string     `json:"reason,omitempty"`

	GroupID    string `json:"group_id,omitempty"`
	GroupType  string `json:"group_type,omitempty"`
	GroupStep  int    `json:"group_step,omitempty"`
	GroupTotal int    `json:"group_total,omitempty"`
}

type ChatStats struct {
	InputTokens    int     `json:"input_tokens"`
	OutputTokens   int     `json:"output_tokens"`
	TokensPerSec   float64 `json:"tokens_per_sec"`
	TimeToFirstSec float64 `json:"time_to_first_sec"`
	ResponseID     string  `json:"response_id,omitempty"`
}

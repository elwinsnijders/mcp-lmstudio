package lmstudio

import "encoding/json"

type ChatRequest struct {
	Model              string      `json:"model"`
	Input              interface{} `json:"input"`
	SystemPrompt       string      `json:"system_prompt,omitempty"`
	Temperature        *float64    `json:"temperature,omitempty"`
	TopP               *float64    `json:"top_p,omitempty"`
	TopK               *int        `json:"top_k,omitempty"`
	MinP               *float64    `json:"min_p,omitempty"`
	RepeatPenalty      *float64    `json:"repeat_penalty,omitempty"`
	MaxOutputTokens    *int        `json:"max_output_tokens,omitempty"`
	Reasoning          string      `json:"reasoning,omitempty"`
	ContextLength      int         `json:"context_length,omitempty"`
	Integrations       interface{} `json:"integrations,omitempty"`
	PreviousResponseID string      `json:"previous_response_id,omitempty"`
	Store              *bool       `json:"store,omitempty"`
	Stream             bool        `json:"stream,omitempty"`
}

type StreamEvent struct {
	Type       string          `json:"type"`
	Delta      string          `json:"delta,omitempty"`
	ResponseID string          `json:"response_id,omitempty"`
	Response   *ChatResponse   `json:"response,omitempty"`
	RawData    json.RawMessage `json:"-"`
}

type ChatResponse struct {
	ModelInstanceID string     `json:"model_instance_id"`
	Output          []Output   `json:"output"`
	Stats           Stats      `json:"stats"`
	ResponseID      string     `json:"response_id,omitempty"`
}

type Output struct {
	Type         string          `json:"type"`
	Content      string          `json:"content,omitempty"`
	Tool         string          `json:"tool,omitempty"`
	Arguments    json.RawMessage `json:"arguments,omitempty"`
	Output       string          `json:"output,omitempty"`
	ProviderInfo json.RawMessage `json:"provider_info,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

type Stats struct {
	InputTokens           int     `json:"input_tokens"`
	TotalOutputTokens     int     `json:"total_output_tokens"`
	ReasoningOutputTokens int     `json:"reasoning_output_tokens"`
	TokensPerSecond       float64 `json:"tokens_per_second"`
	TimeToFirstTokenSec   float64 `json:"time_to_first_token_seconds"`
	ModelLoadTimeSec      float64 `json:"model_load_time_seconds,omitempty"`
}

type Model struct {
	Type             string           `json:"type"`
	Publisher        string           `json:"publisher"`
	Key              string           `json:"key"`
	DisplayName      string           `json:"display_name"`
	Architecture     *string          `json:"architecture,omitempty"`
	Quantization     *Quantization    `json:"quantization"`
	SizeBytes        int64            `json:"size_bytes"`
	ParamsString     *string          `json:"params_string"`
	LoadedInstances  []LoadedInstance `json:"loaded_instances"`
	MaxContextLength int              `json:"max_context_length"`
	Format           *string          `json:"format"`
	Capabilities     *Capabilities    `json:"capabilities,omitempty"`
	Description      *string          `json:"description,omitempty"`
}

type Quantization struct {
	Name          string  `json:"name"`
	BitsPerWeight float64 `json:"bits_per_weight"`
}

type LoadedInstance struct {
	ID     string               `json:"id"`
	Config LoadedInstanceConfig `json:"config"`
}

type LoadedInstanceConfig struct {
	ContextLength       int   `json:"context_length"`
	EvalBatchSize       *int  `json:"eval_batch_size,omitempty"`
	FlashAttention      *bool `json:"flash_attention,omitempty"`
	NumExperts          *int  `json:"num_experts,omitempty"`
	OffloadKVCacheToGPU *bool `json:"offload_kv_cache_to_gpu,omitempty"`
}

type Capabilities struct {
	Vision            bool `json:"vision"`
	TrainedForToolUse bool `json:"trained_for_tool_use"`
}

type ModelsResponse struct {
	Models []Model `json:"models"`
}

type PluginIntegration struct {
	Type         string   `json:"type"`
	ID           string   `json:"id"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
}

type EphemeralMCPIntegration struct {
	Type         string            `json:"type"`
	ServerLabel  string            `json:"server_label"`
	ServerURL    string            `json:"server_url"`
	AllowedTools []string          `json:"allowed_tools,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

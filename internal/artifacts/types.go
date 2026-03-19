package artifacts

import (
	"encoding/json"
	"time"
)

type Artifact struct {
	SessionID    string          `json:"session_id"`
	Sequence     int             `json:"sequence"`
	Tool         string          `json:"tool"`
	Arguments    json.RawMessage `json:"arguments"`
	FilePath     string          `json:"file_path,omitempty"`
	ContentSize  int             `json:"content_size"`
	ProviderInfo json.RawMessage `json:"provider_info,omitempty"`
	Timestamp    time.Time       `json:"timestamp"`
}

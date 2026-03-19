package chatlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Writer struct {
	dir string
	mu  sync.Mutex
}

func NewWriter(dir string) (*Writer, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating chatlog dir: %w", err)
	}
	return &Writer{dir: dir}, nil
}

func (w *Writer) Dir() string {
	return w.dir
}

func (w *Writer) Write(event ChatEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling chat event: %w", err)
	}
	data = append(data, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	path := filepath.Join(w.dir, event.SessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening chatlog %s: %w", path, err)
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func (w *Writer) WriteUserMessage(sessionID, content string) error {
	return w.Write(ChatEvent{
		Type:      EventUserMessage,
		SessionID: sessionID,
		Content:   content,
	})
}

func (w *Writer) WriteDelta(sessionID, content string) error {
	return w.Write(ChatEvent{
		Type:      EventAIDelta,
		SessionID: sessionID,
		Content:   content,
	})
}

func (w *Writer) WriteComplete(sessionID, content string, stats *ChatStats) error {
	return w.Write(ChatEvent{
		Type:      EventAIComplete,
		SessionID: sessionID,
		Content:   content,
		Stats:     stats,
	})
}

func (w *Writer) WriteToolUse(sessionID, tool, content string) error {
	return w.Write(ChatEvent{
		Type:      EventToolUse,
		SessionID: sessionID,
		Tool:      tool,
		Content:   content,
	})
}

func (w *Writer) WriteError(sessionID, content string) error {
	return w.Write(ChatEvent{
		Type:      EventError,
		SessionID: sessionID,
		Content:   content,
	})
}

func (w *Writer) WriteReasoningStart(sessionID string) error {
	return w.Write(ChatEvent{
		Type:      EventReasoningStart,
		SessionID: sessionID,
	})
}

func (w *Writer) WriteReasoningDelta(sessionID, content string) error {
	return w.Write(ChatEvent{
		Type:      EventReasoningDelta,
		SessionID: sessionID,
		Content:   content,
	})
}

func (w *Writer) WriteReasoningEnd(sessionID string) error {
	return w.Write(ChatEvent{
		Type:      EventReasoningEnd,
		SessionID: sessionID,
	})
}

func (w *Writer) WriteStatus(sessionID, phase string, progress float64) error {
	p := progress
	return w.Write(ChatEvent{
		Type:      EventStatus,
		SessionID: sessionID,
		Phase:     phase,
		Progress:  &p,
	})
}

func (w *Writer) WriteToolCallStart(sessionID, tool string) error {
	return w.Write(ChatEvent{
		Type:      EventToolCallStart,
		SessionID: sessionID,
		Tool:      tool,
	})
}

func (w *Writer) WriteToolCallResult(sessionID, tool, arguments, output, reason string, success bool) error {
	return w.Write(ChatEvent{
		Type:      EventToolCallResult,
		SessionID: sessionID,
		Tool:      tool,
		Arguments: arguments,
		Output:    output,
		Reason:    reason,
		Success:   &success,
	})
}

// ReadAll reads all chat events for a session.
func ReadAll(dir, sessionID string) ([]ChatEvent, error) {
	path := filepath.Join(dir, sessionID+".jsonl")
	return ReadFile(path)
}

// ReadFile reads all chat events from a JSONL file.
func ReadFile(path string) ([]ChatEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading chatlog: %w", err)
	}

	var events []ChatEvent
	start := 0
	for i := range data {
		if data[i] == '\n' {
			line := data[start:i]
			start = i + 1
			if len(line) == 0 {
				continue
			}
			var ev ChatEvent
			if json.Unmarshal(line, &ev) == nil {
				events = append(events, ev)
			}
		}
	}
	if start < len(data) {
		var ev ChatEvent
		if json.Unmarshal(data[start:], &ev) == nil {
			events = append(events, ev)
		}
	}
	return events, nil
}

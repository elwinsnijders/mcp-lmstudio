package chatlog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Defaults for UI loads (Live View / archive) — tail only to avoid blocking the main thread.
const (
	DefaultRecentMaxBytes  = 1024 * 1024 // 1 MiB
	DefaultRecentMaxEvents = 1500
)

// ReadRecent reads the last portion of a session chatlog: at most maxBytes from the end of the file,
// then parses JSONL lines and returns at most the last maxEvents events.
// Smaller files are read in full.
func ReadRecent(dir, sessionID string, maxBytes int64, maxEvents int) ([]ChatEvent, error) {
	path := filepath.Join(dir, sessionID+".jsonl")
	return ReadRecentFile(path, maxBytes, maxEvents)
}

// ReadRecentFile is like ReadRecent but takes an explicit file path.
func ReadRecentFile(path string, maxBytes int64, maxEvents int) ([]ChatEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening chatlog: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return nil, nil
	}

	var raw []byte
	if size <= maxBytes {
		raw, err = io.ReadAll(f)
		if err != nil {
			return nil, err
		}
	} else {
		if _, err := f.Seek(size-maxBytes, io.SeekStart); err != nil {
			return nil, err
		}
		raw, err = io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		// Skip first partial line (incomplete JSON at chunk start).
		for i := 0; i < len(raw); i++ {
			if raw[i] == '\n' {
				raw = raw[i+1:]
				break
			}
		}
	}

	events := parseJSONLBytes(raw)
	if len(events) > maxEvents {
		events = events[len(events)-maxEvents:]
	}
	return events, nil
}

func parseJSONLBytes(data []byte) []ChatEvent {
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
	return events
}

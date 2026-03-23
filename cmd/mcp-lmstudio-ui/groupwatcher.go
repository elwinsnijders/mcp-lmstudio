package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/infinitimeless/lmstudio-mcp/internal/chatlog"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type GroupStepEvent struct {
	StepIdx    int      `json:"stepIdx"`
	TotalSteps int      `json:"totalSteps"`
	SessionID  string   `json:"sessionId"`
	Type       string   `json:"type"`
	GroupInfo  GroupDTO `json:"groupInfo"`
}

type GroupDoneEvent struct {
	GroupInfo GroupDTO `json:"groupInfo"`
}

type GroupWatcher struct {
	mu          sync.Mutex
	ctx         context.Context
	dataDir     string
	groupID     string
	knownLen    int
	currentPath string
	currentFile *os.File
	currentOff  int64
	done        chan struct{}
	running     bool
	finished    bool
	logFile     *os.File
}

func NewGroupWatcher() *GroupWatcher {
	return &GroupWatcher{}
}

func (w *GroupWatcher) log(format string, args ...interface{}) {
	if w.logFile == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(w.logFile, "%s %s\n", ts, msg)
	w.logFile.Sync()
}

func (w *GroupWatcher) Start(ctx context.Context, dataDir, groupID string, initialLen int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.stopLocked()
	}

	w.ctx = ctx
	w.dataDir = dataDir
	w.groupID = groupID
	w.knownLen = initialLen
	w.currentPath = ""
	w.currentFile = nil
	w.currentOff = 0
	w.done = make(chan struct{})
	w.running = true
	w.finished = false

	logPath := filepath.Join(dataDir, "groupwatcher.log")
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		w.logFile = lf
	}

	w.log("START group=%s initialLen=%d", groupID, initialLen)

	if initialLen > 0 {
		g := readGroupFile(dataDir, groupID)
		if g != nil && len(g.SessionIDs) >= initialLen {
			lastSID := g.SessionIDs[initialLen-1]
			path := filepath.Join(dataDir, "chatlogs", lastSID+".jsonl")
			w.currentPath = path
			f, err := os.Open(path)
			if err == nil {
				end, _ := f.Seek(0, io.SeekEnd)
				w.currentFile = f
				w.currentOff = end
				w.log("  tailing session[%d]=%s from offset=%d", initialLen-1, lastSID, end)
			} else {
				w.log("  WARN: cannot open chatlog for %s: %v", lastSID, err)
			}
		} else {
			w.log("  WARN: readGroupFile returned nil or too few sessions")
		}
	}

	go w.poll()
}

func (w *GroupWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopLocked()
}

func (w *GroupWatcher) stopLocked() {
	if !w.running {
		return
	}
	w.log("STOP")
	close(w.done)
	w.running = false
	if w.currentFile != nil {
		w.currentFile.Close()
		w.currentFile = nil
	}
	if w.logFile != nil {
		w.logFile.Close()
		w.logFile = nil
	}
}

func (w *GroupWatcher) poll() {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						w.log("PANIC in tick: %v", r)
					}
				}()
				w.tick()
			}()
		}
	}
}

func (w *GroupWatcher) tick() {
	w.mu.Lock()
	if w.finished || !w.running {
		w.mu.Unlock()
		return
	}
	ctx := w.ctx
	dataDir := w.dataDir
	groupID := w.groupID
	knownLen := w.knownLen
	w.mu.Unlock()

	g := readGroupFile(dataDir, groupID)
	if g == nil {
		w.log("tick: readGroupFile returned nil")
		return
	}

	newLen := len(g.SessionIDs)

	if newLen > knownLen {
		nextIdx := knownLen
		sid := g.SessionIDs[nextIdx]
		chatPath := filepath.Join(dataDir, "chatlogs", sid+".jsonl")

		w.log("tick: NEW session[%d]=%s (group has %d, known %d, status=%s)", nextIdx, sid, newLen, knownLen, g.Status)

		w.readAndEmitNewLines(ctx)

		w.mu.Lock()
		if w.currentFile != nil {
			w.currentFile.Close()
			w.currentFile = nil
			w.currentPath = ""
		}
		w.mu.Unlock()

		dto := rawGroupToDTO(g)
		wailsRuntime.EventsEmit(ctx, "group:step", GroupStepEvent{
			StepIdx:    nextIdx,
			TotalSteps: g.TotalSteps,
			SessionID:  sid,
			Type:       g.Type,
			GroupInfo:  dto,
		})
		w.log("  emitted group:step idx=%d", nextIdx)

		f, err := os.Open(chatPath)
		if err == nil {
			n := w.emitAllFromReader(ctx, f)
			off, _ := f.Seek(0, io.SeekCurrent)
			w.mu.Lock()
			w.currentFile = f
			w.currentOff = off
			w.currentPath = chatPath
			w.knownLen = nextIdx + 1
			w.mu.Unlock()
			w.log("  opened chatlog, emitted %d events, offset=%d, knownLen=%d", n, off, nextIdx+1)
		} else {
			w.mu.Lock()
			w.currentFile = nil
			w.currentOff = 0
			w.currentPath = chatPath
			w.knownLen = nextIdx + 1
			w.mu.Unlock()
			w.log("  WARN: chatlog not found yet for %s: %v, knownLen=%d", sid, err, nextIdx+1)
		}
		return
	}

	n := w.readAndEmitNewLines(ctx)
	if n > 0 {
		w.log("tick: tailing emitted %d events (status=%s)", n, g.Status)
	}

	if g.Status == "completed" || g.Status == "failed" {
		w.mu.Lock()
		caught := w.knownLen >= newLen
		w.mu.Unlock()

		if caught {
			n2 := w.readAndEmitNewLines(ctx)
			w.log("tick: DONE status=%s (final flush=%d events)", g.Status, n2)
			dto := rawGroupToDTO(g)
			wailsRuntime.EventsEmit(ctx, "group:done", GroupDoneEvent{
				GroupInfo: dto,
			})
			w.mu.Lock()
			w.finished = true
			w.mu.Unlock()
		} else {
			w.log("tick: group done but not caught up yet (known=%d, total=%d)", w.knownLen, newLen)
		}
	}
}

func skipGroupEvent(eventType string) bool {
	return eventType == chatlog.EventGroupStart ||
		eventType == chatlog.EventGroupStep ||
		eventType == chatlog.EventGroupComplete
}

func (w *GroupWatcher) emitAllFromReader(ctx context.Context, f *os.File) int {
	const maxBuf = 2 * 1024 * 1024
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, maxBuf), maxBuf)
	count := 0
	for scanner.Scan() {
		var event chatlog.ChatEvent
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Type != "" {
			if !skipGroupEvent(event.Type) {
				wailsRuntime.EventsEmit(ctx, "chat:event", event)
				count++
			}
		}
	}
	return count
}

func (w *GroupWatcher) emitFullChatlog(ctx context.Context, path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	return w.emitAllFromReader(ctx, f)
}

func (w *GroupWatcher) readAndEmitNewLines(ctx context.Context) int {
	w.mu.Lock()
	path := w.currentPath
	f := w.currentFile
	currentOff := w.currentOff
	w.mu.Unlock()

	if path == "" {
		return 0
	}

	if f == nil {
		var err error
		f, err = os.Open(path)
		if err != nil {
			return 0
		}
		w.mu.Lock()
		w.currentFile = f
		w.currentOff = 0
		currentOff = 0
		w.mu.Unlock()
	}

	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	if info.Size() < currentOff {
		w.mu.Lock()
		if w.currentFile != nil {
			w.currentFile.Close()
		}
		f, err = os.Open(path)
		if err != nil {
			w.currentFile = nil
			w.currentOff = 0
			w.mu.Unlock()
			return 0
		}
		w.currentFile = f
		w.currentOff = 0
		currentOff = 0
		w.mu.Unlock()
	}

	if info.Size() == currentOff {
		return 0
	}

	f.Seek(currentOff, io.SeekStart)
	const maxBuf = 2 * 1024 * 1024
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, maxBuf), maxBuf)
	count := 0
	for scanner.Scan() {
		var event chatlog.ChatEvent
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Type != "" {
			if !skipGroupEvent(event.Type) {
				wailsRuntime.EventsEmit(ctx, "chat:event", event)
				count++
			}
		}
	}
	if scanner.Err() != nil {
		f.Seek(0, io.SeekEnd)
	}
	newOff, _ := f.Seek(0, io.SeekCurrent)
	w.mu.Lock()
	w.currentOff = newOff
	w.mu.Unlock()
	return count
}

func readGroupFile(dataDir, groupID string) *rawGroup {
	path := filepath.Join(dataDir, "sessions", "groups.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var groups []rawGroup
	if json.Unmarshal(data, &groups) != nil {
		return nil
	}
	for i := range groups {
		if groups[i].ID == groupID {
			return &groups[i]
		}
	}
	return nil
}

func rawGroupToDTO(g *rawGroup) GroupDTO {
	return GroupDTO{
		ID:           g.ID,
		Type:         g.Type,
		Status:       g.Status,
		TotalSteps:   g.TotalSteps,
		CurrentStep:  g.CurrentStep,
		Succeeded:    g.Succeeded,
		Failed:       g.Failed,
		SessionIDs:   g.SessionIDs,
		ChainMode:    g.ChainMode,
		StoppedEarly: g.StoppedEarly,
		CreatedAt:    g.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    g.UpdatedAt.Format(time.RFC3339),
	}
}

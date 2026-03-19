package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/infinitimeless/lmstudio-mcp/internal/chatlog"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ChatWatcher struct {
	mu        sync.Mutex
	ctx       context.Context
	path      string
	sessionID string
	file      *os.File
	offset    int64
	done      chan struct{}
	running   bool
}

func NewChatWatcher() *ChatWatcher {
	return &ChatWatcher{}
}

func (w *ChatWatcher) Start(ctx context.Context, path, sessionID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.stopLocked()
	}

	w.ctx = ctx
	w.path = path
	w.sessionID = sessionID
	w.done = make(chan struct{})
	w.running = true

	f, err := os.Open(path)
	if err != nil {
		w.file = nil
		w.offset = 0
	} else {
		w.file = f
		end, _ := f.Seek(0, io.SeekEnd)
		w.offset = end
	}

	go w.poll()
}

func (w *ChatWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopLocked()
}

func (w *ChatWatcher) stopLocked() {
	if !w.running {
		return
	}
	close(w.done)
	w.running = false
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}
}

func (w *ChatWatcher) poll() {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			w.readNewLines()
		}
	}
}

func (w *ChatWatcher) readNewLines() {
	w.mu.Lock()
	path := w.path
	ctx := w.ctx
	w.mu.Unlock()

	if w.file == nil {
		f, err := os.Open(path)
		if err != nil {
			return
		}
		w.mu.Lock()
		w.file = f
		w.offset = 0
		w.mu.Unlock()
	}

	info, err := os.Stat(path)
	if err != nil {
		return
	}

	w.mu.Lock()
	currentOffset := w.offset
	w.mu.Unlock()

	if info.Size() < currentOffset {
		w.mu.Lock()
		if w.file != nil {
			w.file.Close()
		}
		f, err := os.Open(path)
		if err != nil {
			w.file = nil
			w.offset = 0
			w.mu.Unlock()
			return
		}
		w.file = f
		w.offset = 0
		w.mu.Unlock()
		currentOffset = 0
	}

	if info.Size() == currentOffset {
		return
	}

	w.mu.Lock()
	f := w.file
	w.mu.Unlock()
	if f == nil {
		return
	}

	f.Seek(currentOffset, io.SeekStart)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var event chatlog.ChatEvent
		if json.Unmarshal(line, &event) == nil && event.Type != "" {
			wailsRuntime.EventsEmit(ctx, "chat:event", event)
		}
	}
	newOffset, _ := f.Seek(0, io.SeekCurrent)
	w.mu.Lock()
	w.offset = newOffset
	w.mu.Unlock()
}

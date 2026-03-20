package chatlog

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestReadRecentFile_smallFileFullRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sess.jsonl")
	content := `{"type":"user_message","session_id":"sess","content":"hi"}
{"type":"ai_complete","session_id":"sess","content":"ok"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	ev, err := ReadRecentFile(path, 1024, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev) != 2 {
		t.Fatalf("want 2 events, got %d", len(ev))
	}
}

func TestReadRecentFile_tailAndCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sess.jsonl")
	var b []byte
	for i := 0; i < 30; i++ {
		b = append(b, fmt.Sprintf(`{"type":"user_message","session_id":"s","content":"%d"}`+"\n", i)...)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	// Read only last ~400 bytes; skip partial first line; cap to 5 events
	ev, err := ReadRecentFile(path, 400, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev) != 5 {
		t.Fatalf("want 5 events capped, got %d", len(ev))
	}
}

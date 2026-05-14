package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListCodexSessionsDeduplicatesBySessionID(t *testing.T) {
	workDir := t.TempDir()
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		t.Fatal(err)
	}
	codexHome := t.TempDir()
	sessionsDir := filepath.Join(codexHome, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	olderPath := filepath.Join(sessionsDir, "older.jsonl")
	newerPath := filepath.Join(sessionsDir, "newer.jsonl")
	writeCodexListTestSession(t, olderPath, "dup-thread", absWorkDir, "older summary")
	writeCodexListTestSession(t, newerPath, "dup-thread", absWorkDir, "newer summary")

	olderTime := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	newerTime := olderTime.Add(time.Hour)
	if err := os.Chtimes(olderPath, olderTime, olderTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newerPath, newerTime, newerTime); err != nil {
		t.Fatal(err)
	}

	sessions, err := listCodexSessions(workDir, codexHome)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d, want 1: %#v", len(sessions), sessions)
	}
	if sessions[0].ID != "dup-thread" {
		t.Fatalf("session ID = %q, want dup-thread", sessions[0].ID)
	}
	if sessions[0].Summary != "newer summary" {
		t.Fatalf("summary = %q, want newer summary", sessions[0].Summary)
	}
}

func writeCodexListTestSession(t *testing.T, path, id, cwd, summary string) {
	t.Helper()
	lines := []map[string]any{
		{
			"type": "session_meta",
			"payload": map[string]any{
				"id":  id,
				"cwd": cwd,
			},
		},
		{
			"type": "response_item",
			"payload": map[string]any{
				"role": "user",
				"content": []map[string]string{
					{"type": "input_text", "text": summary},
				},
			},
		},
	}

	var data []byte
	for _, line := range lines {
		b, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, b...)
		data = append(data, '\n')
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

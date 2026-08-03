package bypass

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestNoticeTrackerMarksSessionOnce(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "bypass_notice.json")
	now := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)

	tracker := NewNoticeTracker(
		WithStateFile(stateFile),
		WithTimeFunc(func() time.Time { return now }),
	)

	first, err := tracker.MarkNotified("claude", "sess-1")
	if err != nil {
		t.Fatalf("MarkNotified() error = %v", err)
	}

	if !first {
		t.Error("MarkNotified() = false on first call, want true")
	}

	again, err := tracker.MarkNotified("claude", "sess-1")
	if err != nil {
		t.Fatalf("MarkNotified() error = %v", err)
	}

	if again {
		t.Error("MarkNotified() = true on second call, want false")
	}
}

func TestNoticeTrackerSeparatesSessionsAndProviders(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "bypass_notice.json")
	tracker := NewNoticeTracker(WithStateFile(stateFile))

	if first, _ := tracker.MarkNotified("claude", "sess-1"); !first {
		t.Fatal("MarkNotified(claude, sess-1) = false, want true")
	}

	if first, _ := tracker.MarkNotified("claude", "sess-2"); !first {
		t.Error("MarkNotified(claude, sess-2) = false, want true")
	}

	if first, _ := tracker.MarkNotified("codex", "sess-1"); !first {
		t.Error("MarkNotified(codex, sess-1) = false, want true")
	}
}

func TestNoticeTrackerPrunesStaleSessions(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "bypass_notice.json")
	start := time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC)
	now := start

	tracker := NewNoticeTracker(
		WithStateFile(stateFile),
		WithTimeFunc(func() time.Time { return now }),
		WithRetention(time.Hour),
	)

	if _, err := tracker.MarkNotified("claude", "sess-1"); err != nil {
		t.Fatalf("MarkNotified() error = %v", err)
	}

	now = start.Add(2 * time.Hour)

	first, err := tracker.MarkNotified("claude", "sess-1")
	if err != nil {
		t.Fatalf("MarkNotified() error = %v", err)
	}

	if !first {
		t.Error("MarkNotified() = false after retention elapsed, want true")
	}
}

func TestNoticeTrackerRecoversFromCorruptState(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "bypass_notice.json")
	if err := os.WriteFile(stateFile, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tracker := NewNoticeTracker(WithStateFile(stateFile))

	first, err := tracker.MarkNotified("claude", "sess-1")
	if err != nil {
		t.Fatalf("MarkNotified() error = %v, want nil for corrupt state", err)
	}

	if !first {
		t.Error("MarkNotified() = false for corrupt state, want true")
	}

	// The corrupt file must be repaired, not left in place, or the notice
	// would repeat on every hook invocation forever.
	again, err := tracker.MarkNotified("claude", "sess-1")
	if err != nil {
		t.Fatalf("MarkNotified() error = %v", err)
	}

	if again {
		t.Error("MarkNotified() = true after repair, want false")
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if !json.Valid(data) {
		t.Errorf("state file still invalid JSON after repair: %s", data)
	}
}

func TestNoticeTrackerShowsNoticeWhenStateIsUnreadable(t *testing.T) {
	// A directory where the state file should be: ReadFile fails with an
	// error that persistence cannot recover from.
	stateFile := filepath.Join(t.TempDir(), "bypass_notice.json")
	if err := os.Mkdir(stateFile, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	tracker := NewNoticeTracker(WithStateFile(stateFile))

	first, err := tracker.MarkNotified("claude", "sess-1")
	if err == nil {
		t.Error("MarkNotified() error = nil for unreadable state, want error")
	}

	if !first {
		t.Error("MarkNotified() = false for unreadable state, want true (fail open)")
	}
}

func TestNoticeTrackerLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	tracker := NewNoticeTracker(WithStateFile(filepath.Join(dir, "bypass_notice.json")))

	for i := range 3 {
		if _, err := tracker.MarkNotified("claude", strconv.Itoa(i)); err != nil {
			t.Fatalf("MarkNotified() error = %v", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}

		t.Errorf("state dir holds %v, want only the state file", names)
	}
}

func TestNoticeTrackerHandlesNilReceiver(t *testing.T) {
	var tracker *NoticeTracker

	first, err := tracker.MarkNotified("claude", "sess-1")
	if err != nil {
		t.Fatalf("MarkNotified() error = %v", err)
	}

	if first {
		t.Error("MarkNotified() = true for nil tracker, want false")
	}
}

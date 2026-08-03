package bypass

import (
	"os"
	"path/filepath"
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

	if _, err := tracker.MarkNotified("claude", "sess-1"); err == nil {
		t.Error("MarkNotified() error = nil for corrupt state, want error")
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

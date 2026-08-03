package bypass

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/smykla-skalski/klaudiush/internal/xdg"
)

const (
	// defaultRetention drops notice records for sessions nobody touched in a week.
	defaultRetention = 7 * 24 * time.Hour

	stateFileMode = 0o600
)

// noticeState is the persisted record of sessions already notified.
type noticeState struct {
	Sessions map[string]time.Time `json:"sessions"`
}

// NoticeTracker keeps the bypass notice to once per session.
type NoticeTracker struct {
	stateFile string
	now       func() time.Time
	retention time.Duration
}

// TrackerOption configures a NoticeTracker.
type TrackerOption func(*NoticeTracker)

// WithStateFile overrides the persisted state path.
func WithStateFile(path string) TrackerOption {
	return func(t *NoticeTracker) {
		if path != "" {
			t.stateFile = path
		}
	}
}

// WithTimeFunc overrides the clock used by the tracker.
func WithTimeFunc(fn func() time.Time) TrackerOption {
	return func(t *NoticeTracker) {
		if fn != nil {
			t.now = fn
		}
	}
}

// WithRetention overrides how long notice records are kept.
func WithRetention(retention time.Duration) TrackerOption {
	return func(t *NoticeTracker) {
		if retention > 0 {
			t.retention = retention
		}
	}
}

// NewNoticeTracker creates a tracker backed by the XDG state directory.
func NewNoticeTracker(opts ...TrackerOption) *NoticeTracker {
	tracker := &NoticeTracker{
		stateFile: xdg.BypassNoticeStateFile(),
		now:       time.Now,
		retention: defaultRetention,
	}

	for _, opt := range opts {
		opt(tracker)
	}

	return tracker
}

// MarkNotified records the notice for a provider/session pair and reports
// whether this is the first time it was shown.
func (t *NoticeTracker) MarkNotified(provider, sessionID string) (bool, error) {
	if t == nil {
		return false, nil
	}

	state, err := t.load()
	if err != nil {
		return false, err
	}

	t.prune(state)

	key := provider + ":" + sessionID
	_, seen := state.Sessions[key]
	state.Sessions[key] = t.now()

	if err := t.save(state); err != nil {
		return !seen, err
	}

	return !seen, nil
}

func (t *NoticeTracker) load() (*noticeState, error) {
	// Path comes from trusted configuration, not user input.
	data, err := os.ReadFile(t.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &noticeState{Sessions: make(map[string]time.Time)}, nil
		}

		return nil, errors.Wrap(err, "failed to read bypass notice state")
	}

	if len(data) == 0 {
		return &noticeState{Sessions: make(map[string]time.Time)}, nil
	}

	var state noticeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, errors.Wrap(err, "failed to parse bypass notice state")
	}

	if state.Sessions == nil {
		state.Sessions = make(map[string]time.Time)
	}

	return &state, nil
}

func (t *NoticeTracker) save(state *noticeState) error {
	if err := xdg.EnsureDir(filepath.Dir(t.stateFile)); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal bypass notice state")
	}

	data = append(data, '\n')

	tmpFile := t.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, stateFileMode); err != nil {
		return errors.Wrap(err, "failed to write bypass notice temp file")
	}

	if err := os.Rename(tmpFile, t.stateFile); err != nil {
		_ = os.Remove(tmpFile)

		return errors.Wrap(err, "failed to replace bypass notice state")
	}

	return nil
}

func (t *NoticeTracker) prune(state *noticeState) {
	now := t.now()

	for key, seen := range state.Sessions {
		if !seen.IsZero() && now.Sub(seen) > t.retention {
			delete(state.Sessions, key)
		}
	}
}

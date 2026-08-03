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
//
// Tracking failures report "first" so a broken state file makes the notice
// repeat rather than disappear. Callers should log the error.
func (t *NoticeTracker) MarkNotified(provider, sessionID string) (bool, error) {
	if t == nil {
		return false, nil
	}

	state, err := t.load()
	if err != nil {
		return true, err
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

// load reads the state file. Unreadable content is treated as no state at all,
// so the next save repairs the file instead of leaving it broken forever.
// Only errors that persistence cannot recover from are returned.
func (t *NoticeTracker) load() (*noticeState, error) {
	// Path comes from trusted configuration, not user input.
	data, err := os.ReadFile(t.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyNoticeState(), nil
		}

		return nil, errors.Wrap(err, "failed to read bypass notice state")
	}

	if len(data) == 0 {
		return emptyNoticeState(), nil
	}

	var state noticeState
	if err := json.Unmarshal(data, &state); err != nil {
		// Deliberate: unparseable content is discarded so the next save
		// repairs the file. Returning the error here would suppress the
		// notice forever without ever rewriting the bad state.
		//nolint:nilerr // recovering from corrupt state is the point
		return emptyNoticeState(), nil
	}

	if state.Sessions == nil {
		state.Sessions = make(map[string]time.Time)
	}

	return &state, nil
}

// save atomically replaces the state file. The temp file carries a random
// suffix so concurrent klaudiush processes never write the same one.
func (t *NoticeTracker) save(state *noticeState) error {
	dir := filepath.Dir(t.stateFile)
	if err := xdg.EnsureDir(dir); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal bypass notice state")
	}

	data = append(data, '\n')

	tmpFile, err := os.CreateTemp(dir, "bypass_notice-*.json")
	if err != nil {
		return errors.Wrap(err, "failed to create bypass notice temp file")
	}

	tmpPath := tmpFile.Name()

	if err := writeAndClose(tmpFile, data); err != nil {
		_ = os.Remove(tmpPath)

		return err
	}

	if err := os.Rename(tmpPath, t.stateFile); err != nil {
		_ = os.Remove(tmpPath)

		return errors.Wrap(err, "failed to replace bypass notice state")
	}

	return nil
}

// writeAndClose writes data to f and closes it, reporting the first failure.
func writeAndClose(f *os.File, data []byte) (err error) {
	defer func() {
		closeErr := f.Close()
		if err == nil && closeErr != nil {
			err = errors.Wrap(closeErr, "failed to close bypass notice temp file")
		}
	}()

	if chmodErr := f.Chmod(stateFileMode); chmodErr != nil {
		return errors.Wrap(chmodErr, "failed to set bypass notice temp file mode")
	}

	if _, writeErr := f.Write(data); writeErr != nil {
		return errors.Wrap(writeErr, "failed to write bypass notice temp file")
	}

	return nil
}

// emptyNoticeState returns a state with no sessions recorded.
func emptyNoticeState() *noticeState {
	return &noticeState{Sessions: make(map[string]time.Time)}
}

func (t *NoticeTracker) prune(state *noticeState) {
	now := t.now()

	for key, seen := range state.Sessions {
		if !seen.IsZero() && now.Sub(seen) > t.retention {
			delete(state.Sessions, key)
		}
	}
}

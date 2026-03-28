// Package updatecheck provides automatic update checking with cached state.
package updatecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/smykla-skalski/klaudiush/internal/xdg"
)

const (
	stateFilePermissions = 0o600
	stateDirPermissions  = 0o700
)

// State represents the persisted update check state.
type State struct {
	LastChecked     time.Time `json:"last_checked"`
	LatestVersion   string    `json:"latest_version"`
	NotifiedVersion string    `json:"notified_version"`
}

// LoadState reads the state file and returns the parsed state.
// Returns a zero-value State (not an error) when the file is missing or corrupt.
func LoadState(path string) State {
	// Path comes from trusted configuration, not user input.
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is from config/xdg
	if err != nil {
		return State{}
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}
	}

	return state
}

// SaveState atomically writes the state to the given path.
// Creates parent directories if needed.
func SaveState(path string, s *State) error {
	dir := filepath.Dir(path)
	if err := xdg.EnsureDir(dir); err != nil {
		return errors.Wrap(err, "creating state directory")
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshaling state")
	}

	// Write to temp file first for atomic operation
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, stateFilePermissions); err != nil {
		return errors.Wrap(err, "writing temp state file")
	}

	// Rename for atomic replace
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)

		return errors.Wrap(err, "renaming state file")
	}

	return nil
}

// DefaultStatePath returns the default state file path.
func DefaultStatePath() string {
	return xdg.UpdateCheckStateFile()
}

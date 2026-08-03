// Package config provides configuration schema types for klaudiush validators.
package config

import (
	"strings"
	"time"
)

// BypassPermissionsConfig controls klaudiush behavior when the session runs in a
// permission mode that skips approval prompts, such as Claude Code's
// --dangerously-skip-permissions or Codex's --dangerously-bypass-approvals-and-sandbox.
//
// Validation stays on in those modes unless SkipValidation is set.
type BypassPermissionsConfig struct {
	// SkipValidation turns klaudiush off while a bypass permission mode is active.
	// Default: false - every hook is validated regardless of permission mode.
	SkipValidation *bool `json:"skip_validation,omitempty" koanf:"skip_validation" toml:"skip_validation,omitempty"`

	// Notify controls the user-only notice shown while klaudiush keeps validating
	// a session that runs without approval prompts.
	// Default: true.
	Notify *bool `json:"notify,omitempty" koanf:"notify" toml:"notify,omitempty"`

	// Modes lists the permission mode values treated as bypass modes.
	// Empty means the built-in provider defaults.
	Modes []string `json:"modes,omitempty" koanf:"modes" toml:"modes,omitempty"`

	// Reason explains why validation is skipped.
	Reason string `json:"reason,omitempty" koanf:"reason" toml:"reason,omitempty"`

	// SkippedAt is the RFC3339 timestamp when skipping was enabled.
	SkippedAt string `json:"skipped_at,omitempty" koanf:"skipped_at" toml:"skipped_at,omitempty"`

	// ExpiresAt is the RFC3339 timestamp when skipping stops applying.
	// Empty means permanent.
	ExpiresAt string `json:"expires_at,omitempty" koanf:"expires_at" toml:"expires_at,omitempty"`

	// SkippedBy records who enabled skipping (for example "cli").
	SkippedBy string `json:"skipped_by,omitempty" koanf:"skipped_by" toml:"skipped_by,omitempty"`
}

// IsSkipValidation reports whether validation is skipped in bypass permission modes.
// An expired entry no longer skips.
func (b *BypassPermissionsConfig) IsSkipValidation() bool {
	if b == nil || b.SkipValidation == nil || !*b.SkipValidation {
		return false
	}

	return !b.IsExpired()
}

// IsNotifyEnabled reports whether the user-only notice is emitted. Defaults to true.
func (b *BypassPermissionsConfig) IsNotifyEnabled() bool {
	if b == nil || b.Notify == nil {
		return true
	}

	return *b.Notify
}

// IsExpired returns true if the skip entry has an expiry and it has passed.
func (b *BypassPermissionsConfig) IsExpired() bool {
	if b == nil || b.ExpiresAt == "" {
		return false
	}

	t, err := time.Parse(time.RFC3339, b.ExpiresAt)
	if err != nil {
		return false
	}

	return time.Now().After(t)
}

// GetModes returns the configured bypass permission modes, or nil when the
// built-in provider defaults apply. Entries are comma-split so a single
// environment variable can carry several modes.
func (b *BypassPermissionsConfig) GetModes() []string {
	if b == nil || len(b.Modes) == 0 {
		return nil
	}

	modes := make([]string, 0, len(b.Modes))

	for _, entry := range b.Modes {
		for mode := range strings.SplitSeq(entry, ",") {
			if mode = strings.TrimSpace(mode); mode != "" {
				modes = append(modes, mode)
			}
		}
	}

	return modes
}

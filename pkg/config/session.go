package config

import "time"

// Default values for session configuration.
const (
	// DefaultSessionStateFile is the default state file path.
	DefaultSessionStateFile = "~/.klaudiush/session_state.json"

	// DefaultMaxSessionAge is the default maximum session age.
	DefaultMaxSessionAge = 24 * time.Hour

	// DefaultSessionAuditLogFile is the default session audit log file path.
	DefaultSessionAuditLogFile = "~/.klaudiush/session_audit.jsonl"

	// DefaultSessionAuditMaxSizeMB is the default max audit log size in MB.
	DefaultSessionAuditMaxSizeMB = 10

	// DefaultSessionAuditMaxAgeDays is the default max age for audit entries.
	DefaultSessionAuditMaxAgeDays = 30

	// DefaultSessionAuditMaxBackups is the default max number of backup files.
	DefaultSessionAuditMaxBackups = 5
)

// SessionConfig contains configuration for session tracking.
// Session tracking enables fast-fail for subsequent commands after a
// blocking error occurs in the same Claude Code session.
type SessionConfig struct {
	// Enabled controls whether session tracking is active.
	// Default: true
	Enabled *bool `json:"enabled,omitempty" koanf:"enabled" toml:"enabled"`

	// StateFile is the path to the session state file.
	// Default: "~/.klaudiush/session_state.json"
	StateFile string `json:"state_file,omitempty" koanf:"state_file" toml:"state_file"`

	// MaxSessionAge is the maximum age before a session is expired.
	// Default: "24h"
	MaxSessionAge Duration `json:"max_session_age,omitempty" koanf:"max_session_age" toml:"max_session_age"`

	// Audit contains audit logging configuration for session operations.
	Audit *SessionAuditConfig `json:"audit,omitempty" koanf:"audit" toml:"audit"`
}

// SessionAuditConfig contains configuration for session audit logging.
type SessionAuditConfig struct {
	// Enabled controls whether session audit logging is active.
	// Default: true
	Enabled *bool `json:"enabled,omitempty" koanf:"enabled" toml:"enabled"`

	// LogFile is the path to the session audit log file.
	// Default: "~/.klaudiush/session_audit.jsonl"
	LogFile string `json:"log_file,omitempty" koanf:"log_file" toml:"log_file"`

	// MaxSizeMB is the maximum size of the audit log before rotation.
	// Default: 10
	MaxSizeMB int `json:"max_size_mb,omitempty" koanf:"max_size_mb" toml:"max_size_mb"`

	// MaxAgeDays is the maximum age of audit entries before cleanup.
	// Default: 30
	MaxAgeDays int `json:"max_age_days,omitempty" koanf:"max_age_days" toml:"max_age_days"`

	// MaxBackups is the maximum number of backup files to retain.
	// Default: 5
	MaxBackups int `json:"max_backups,omitempty" koanf:"max_backups" toml:"max_backups"`
}

// IsEnabled returns true if session tracking is enabled.
// Returns true if Enabled is nil (default behavior).
func (s *SessionConfig) IsEnabled() bool {
	if s == nil || s.Enabled == nil {
		return true
	}

	return *s.Enabled
}

// GetStateFile returns the state file path.
// Returns DefaultSessionStateFile if StateFile is empty.
func (s *SessionConfig) GetStateFile() string {
	if s == nil || s.StateFile == "" {
		return DefaultSessionStateFile
	}

	return s.StateFile
}

// GetMaxSessionAge returns the maximum session age as a time.Duration.
// Returns DefaultMaxSessionAge if MaxSessionAge is zero.
func (s *SessionConfig) GetMaxSessionAge() time.Duration {
	if s == nil || s.MaxSessionAge == 0 {
		return DefaultMaxSessionAge
	}

	return time.Duration(s.MaxSessionAge)
}

// GetAudit returns the audit config, creating defaults if nil.
func (s *SessionConfig) GetAudit() *SessionAuditConfig {
	if s == nil || s.Audit == nil {
		return &SessionAuditConfig{}
	}

	return s.Audit
}

// IsAuditEnabled returns true if session audit logging is enabled.
// Returns true if Enabled is nil (default behavior).
func (a *SessionAuditConfig) IsAuditEnabled() bool {
	if a == nil || a.Enabled == nil {
		return true
	}

	return *a.Enabled
}

// GetLogFile returns the audit log file path.
// Returns DefaultSessionAuditLogFile if LogFile is empty.
func (a *SessionAuditConfig) GetLogFile() string {
	if a == nil || a.LogFile == "" {
		return DefaultSessionAuditLogFile
	}

	return a.LogFile
}

// GetMaxSizeMB returns the max file size in MB.
// Returns DefaultSessionAuditMaxSizeMB if MaxSizeMB is zero.
func (a *SessionAuditConfig) GetMaxSizeMB() int {
	if a == nil || a.MaxSizeMB == 0 {
		return DefaultSessionAuditMaxSizeMB
	}

	return a.MaxSizeMB
}

// GetMaxAgeDays returns the max age in days.
// Returns DefaultSessionAuditMaxAgeDays if MaxAgeDays is zero.
func (a *SessionAuditConfig) GetMaxAgeDays() int {
	if a == nil || a.MaxAgeDays == 0 {
		return DefaultSessionAuditMaxAgeDays
	}

	return a.MaxAgeDays
}

// GetMaxBackups returns the max number of backup files.
// Returns DefaultSessionAuditMaxBackups if MaxBackups is zero.
func (a *SessionAuditConfig) GetMaxBackups() int {
	if a == nil || a.MaxBackups == 0 {
		return DefaultSessionAuditMaxBackups
	}

	return a.MaxBackups
}

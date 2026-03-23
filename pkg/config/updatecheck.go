// Package config provides configuration schema types for klaudiush validators.
package config

import "time"

const (
	// DefaultUpdateCheckInterval is the default interval between update checks.
	DefaultUpdateCheckInterval = 24 * time.Hour
)

// UpdateCheckConfig contains configuration for the automatic update check system.
//
// Example configuration:
//
//	[update_check]
//	enabled = true
//	check_interval = "24h"
//	notify = true
type UpdateCheckConfig struct {
	// Enabled controls whether automatic update checks are performed.
	// Default: true
	Enabled *bool `json:"enabled,omitempty" koanf:"enabled" toml:"enabled,omitempty"`

	// CheckInterval is how often to query GitHub for new releases.
	// Default: "24h"
	CheckInterval Duration `json:"check_interval,omitempty" koanf:"check_interval" toml:"check_interval,omitempty"`

	// Notify controls whether a notification is shown when an update is available.
	// Default: true
	Notify *bool `json:"notify,omitempty" koanf:"notify" toml:"notify,omitempty"`
}

// IsEnabled returns whether update checks are enabled.
func (c *UpdateCheckConfig) IsEnabled() bool {
	if c == nil || c.Enabled == nil {
		return true
	}

	return *c.Enabled
}

// GetCheckInterval returns the check interval, using default if not set.
func (c *UpdateCheckConfig) GetCheckInterval() time.Duration {
	if c == nil || c.CheckInterval.ToDuration() == 0 {
		return DefaultUpdateCheckInterval
	}

	return c.CheckInterval.ToDuration()
}

// IsNotifyEnabled returns whether update notifications are enabled.
func (c *UpdateCheckConfig) IsNotifyEnabled() bool {
	if c == nil || c.Notify == nil {
		return true
	}

	return *c.Notify
}

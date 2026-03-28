package updatecheck

import (
	"context"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/smykla-skalski/klaudiush/internal/github"
	"github.com/smykla-skalski/klaudiush/internal/updater"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

const (
	// apiTimeout is the maximum time to wait for a GitHub API response.
	apiTimeout = 3 * time.Second
)

// Checker performs cached update checks against GitHub releases.
type Checker struct {
	currentVersion string
	ghClient       github.Client
	cfg            *config.UpdateCheckConfig
	statePath      string
	now            func() time.Time
	log            logger.Logger
}

// Option configures a Checker.
type Option func(*Checker)

// WithStatePath overrides the default state file path.
func WithStatePath(path string) Option {
	return func(c *Checker) {
		c.statePath = path
	}
}

// WithTimeFunc overrides the time source (for testing).
func WithTimeFunc(fn func() time.Time) Option {
	return func(c *Checker) {
		c.now = fn
	}
}

// WithLogger sets the logger.
func WithLogger(log logger.Logger) Option {
	return func(c *Checker) {
		c.log = log
	}
}

// NewChecker creates a new update checker.
func NewChecker(
	currentVersion string,
	ghClient github.Client,
	cfg *config.UpdateCheckConfig,
	opts ...Option,
) *Checker {
	c := &Checker{
		currentVersion: currentVersion,
		ghClient:       ghClient,
		cfg:            cfg,
		statePath:      DefaultStatePath(),
		now:            time.Now,
		log:            logger.NewNoOpLogger(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Check performs the update check and returns a notification message.
// Returns "" if no notification is needed.
func (c *Checker) Check(ctx context.Context) string {
	// Skip dev builds
	if c.currentVersion == "dev" {
		return ""
	}

	if !c.cfg.IsEnabled() {
		return ""
	}

	state := LoadState(c.statePath)

	// Refresh cache if stale
	if c.isStale(state) {
		latest, err := c.fetchLatest(ctx)
		if err != nil {
			c.log.Debug("update check failed", "error", err)

			return ""
		}

		state.LatestVersion = latest
		state.LastChecked = c.now()

		if saveErr := SaveState(c.statePath, &state); saveErr != nil {
			c.log.Debug("failed to save update check state", "error", saveErr)
		}
	}

	if state.LatestVersion == "" {
		return ""
	}

	// Compare versions
	if !c.isNewer(state.LatestVersion) {
		return ""
	}

	if !c.cfg.IsNotifyEnabled() {
		return ""
	}

	// Already notified for this version
	if state.NotifiedVersion == state.LatestVersion {
		return ""
	}

	// Mark as notified and save
	state.NotifiedVersion = state.LatestVersion

	if saveErr := SaveState(c.statePath, &state); saveErr != nil {
		c.log.Debug("failed to save notification state", "error", saveErr)
	}

	return FormatNotification(c.currentVersion, state.LatestVersion)
}

// isStale returns true if the cached state needs refreshing.
func (c *Checker) isStale(state State) bool {
	if state.LastChecked.IsZero() {
		return true
	}

	return c.now().Sub(state.LastChecked) >= c.cfg.GetCheckInterval()
}

// fetchLatest queries GitHub for the latest release tag.
func (c *Checker) fetchLatest(ctx context.Context) (string, error) {
	apiCtx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	release, err := c.ghClient.GetLatestRelease(apiCtx, updater.GitHubOwner, updater.GitHubRepo)
	if err != nil {
		return "", err
	}

	return release.TagName, nil
}

// isNewer returns true if the latest version is newer than the current version.
func (c *Checker) isNewer(latestTag string) bool {
	latest, err := semver.NewVersion(strings.TrimPrefix(latestTag, "v"))
	if err != nil {
		return false
	}

	current, err := semver.NewVersion(strings.TrimPrefix(c.currentVersion, "v"))
	if err != nil {
		return false
	}

	return current.LessThan(latest)
}

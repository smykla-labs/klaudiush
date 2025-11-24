// Package initcmd provides utilities for the init command.
package initcmd

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/smykla-labs/klaudiush/internal/git"
	"github.com/smykla-labs/klaudiush/internal/prompt"
	pkgConfig "github.com/smykla-labs/klaudiush/pkg/config"
)

// ConfigOption represents a configurable option during initialization.
type ConfigOption interface {
	// Name returns the display name of this option.
	Name() string

	// Prompt prompts the user for configuration and applies it to the config.
	Prompt(prompter prompt.Prompter, cfg *pkgConfig.Config) error

	// IsAvailable checks if this option is available (e.g., depends on git repo).
	IsAvailable() bool
}

// SignoffOption configures git commit signoff validation.
type SignoffOption struct{}

// NewSignoffOption creates a new SignoffOption.
func NewSignoffOption() *SignoffOption {
	return &SignoffOption{}
}

// Name returns the display name of this option.
func (*SignoffOption) Name() string {
	return "Git Commit Signoff"
}

// IsAvailable checks if this option is available.
// Signoff is always available, but git config provides a default.
func (*SignoffOption) IsAvailable() bool {
	return true
}

// Prompt prompts the user for signoff configuration.
func (*SignoffOption) Prompt(prompter prompt.Prompter, cfg *pkgConfig.Config) error {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Git Commit Signoff Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("This validates that commits are signed off with the specified name/email.")
	fmt.Println("Leave empty to skip signoff validation.")
	fmt.Println()

	// Get default signoff from git config
	defaultSignoff := ""

	if git.IsInGitRepo() {
		if cfgReader, err := git.NewConfigReader(); err == nil {
			if signoff, err := cfgReader.GetSignoff(); err == nil {
				defaultSignoff = signoff
			}
		}
	}

	// Prompt for signoff
	signoff, err := prompter.Input(
		"Signoff (Name <email>)",
		defaultSignoff,
	)
	if err != nil {
		if !errors.Is(err, prompt.ErrEmptyInput) {
			return errors.Wrap(err, "failed to read signoff")
		}

		signoff = "" // Allow empty signoff
	}

	// Apply to config
	applySignoffConfig(cfg, signoff)

	fmt.Println()

	return nil
}

// applySignoffConfig applies the signoff configuration to the config.
func applySignoffConfig(cfg *pkgConfig.Config, signoff string) {
	if signoff == "" {
		fmt.Println("✓ Signoff validation disabled")

		return
	}

	if cfg.Validators == nil {
		cfg.Validators = &pkgConfig.ValidatorsConfig{}
	}

	if cfg.Validators.Git == nil {
		cfg.Validators.Git = &pkgConfig.GitConfig{}
	}

	if cfg.Validators.Git.Commit == nil {
		cfg.Validators.Git.Commit = &pkgConfig.CommitValidatorConfig{}
	}

	if cfg.Validators.Git.Commit.Message == nil {
		cfg.Validators.Git.Commit.Message = &pkgConfig.CommitMessageConfig{}
	}

	cfg.Validators.Git.Commit.Message.ExpectedSignoff = signoff

	fmt.Printf("✓ Signoff configured: %s\n", signoff)
}

// BellNotificationOption configures notification bell settings.
type BellNotificationOption struct{}

// NewBellNotificationOption creates a new BellNotificationOption.
func NewBellNotificationOption() *BellNotificationOption {
	return &BellNotificationOption{}
}

// Name returns the display name of this option.
func (*BellNotificationOption) Name() string {
	return "Notification Bell"
}

// IsAvailable checks if this option is available.
func (*BellNotificationOption) IsAvailable() bool {
	return true
}

// Prompt prompts the user for bell notification configuration.
func (*BellNotificationOption) Prompt(prompter prompt.Prompter, cfg *pkgConfig.Config) error {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Notification Bell Configuration")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("This sends a bell character to trigger terminal notifications.")
	fmt.Println("Useful for getting notified about permission prompts.")
	fmt.Println()

	// Prompt for bell enabled
	enabled, err := prompter.Confirm(
		"Enable notification bell",
		true,
	)
	if err != nil {
		return errors.Wrap(err, "failed to read bell confirmation")
	}

	// Apply to config
	if cfg.Validators == nil {
		cfg.Validators = &pkgConfig.ValidatorsConfig{}
	}

	if cfg.Validators.Notification == nil {
		cfg.Validators.Notification = &pkgConfig.NotificationConfig{}
	}

	if cfg.Validators.Notification.Bell == nil {
		cfg.Validators.Notification.Bell = &pkgConfig.BellValidatorConfig{}
	}

	cfg.Validators.Notification.Bell.Enabled = &enabled

	if enabled {
		fmt.Println("✓ Notification bell enabled")
	} else {
		fmt.Println("✓ Notification bell disabled")
	}

	fmt.Println()

	return nil
}

// GetDefaultOptions returns the default set of configuration options.
func GetDefaultOptions() []ConfigOption {
	return []ConfigOption{
		NewSignoffOption(),
		NewBellNotificationOption(),
		// Future options can be added here:
		// NewCommitMessageFormatOption(),
		// NewPRValidationOption(),
		// NewBranchNamingOption(),
		// etc.
	}
}

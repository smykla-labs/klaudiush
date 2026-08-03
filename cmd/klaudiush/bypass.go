// Package main provides the CLI entry point for klaudiush.
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"

	internalconfig "github.com/smykla-skalski/klaudiush/internal/config"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
)

// Bypass command flags.
var (
	bypassReason   string
	bypassDuration string
	bypassGlobal   bool
	bypassAll      bool
)

var bypassCmd = &cobra.Command{
	Use:   "bypass",
	Short: "Control validation when approval prompts are off",
	Long: `Control validation when the session runs without approval prompts.

klaudiush validates every hook by default, including sessions started with
--dangerously-skip-permissions (Claude), --dangerously-bypass-approvals-and-sandbox
(Codex), or --yolo (Gemini).

Examples:
  klaudiush bypass status              # Show the effective setting
  klaudiush bypass skip                # Stop validating in bypass modes
  klaudiush bypass skip --global       # Same, for every project
  klaudiush bypass enforce             # Restore the default
  klaudiush bypass notify off          # Keep validating, hide the reminder`,
	RunE: runBypassStatus,
}

var bypassStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether validation runs in bypass permission modes",
	RunE:  runBypassStatus,
}

var bypassSkipCmd = &cobra.Command{
	Use:   "skip",
	Short: "Skip validation while approval prompts are off",
	Long: `Skip validation while the session runs without approval prompts.

Examples:
  klaudiush bypass skip --reason "throwaway spike"
  klaudiush bypass skip --duration 4h --reason "migration sprint"
  klaudiush bypass skip --global`,
	Args: cobra.NoArgs,
	RunE: runBypassSkip,
}

var bypassEnforceCmd = &cobra.Command{
	Use:   "enforce",
	Short: "Validate even while approval prompts are off (default)",
	Args:  cobra.NoArgs,
	RunE:  runBypassEnforce,
}

var bypassNotifyCmd = &cobra.Command{
	Use:       "notify on|off",
	Short:     "Toggle the reminder shown in bypass permission modes",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{valueOn, valueOff},
	RunE:      runBypassNotify,
}

func init() {
	rootCmd.AddCommand(bypassCmd)

	bypassCmd.AddCommand(bypassStatusCmd)
	bypassCmd.AddCommand(bypassSkipCmd)
	bypassCmd.AddCommand(bypassEnforceCmd)
	bypassCmd.AddCommand(bypassNotifyCmd)

	bypassSkipCmd.Flags().StringVarP(
		&bypassReason, "reason", "r", "", "Why validation is skipped",
	)
	bypassSkipCmd.Flags().StringVarP(
		&bypassDuration, "duration", "d", "", "Duration before the skip expires (e.g., 4h, 7d)",
	)
	bypassSkipCmd.Flags().BoolVar(
		&bypassGlobal, "global", false, "Write to global config instead of project",
	)

	bypassEnforceCmd.Flags().BoolVar(
		&bypassGlobal, "global", false, "Write to global config instead of project",
	)
	bypassNotifyCmd.Flags().BoolVar(
		&bypassGlobal, "global", false, "Write to global config instead of project",
	)

	bypassStatusCmd.Flags().BoolVar(
		&bypassGlobal, "global", false, "Show only the global setting",
	)
	bypassStatusCmd.Flags().BoolVar(
		&bypassAll, "all", false, "Show both global and project settings",
	)
}

func runBypassSkip(_ *cobra.Command, _ []string) error {
	var expiresAt string

	if bypassDuration != "" {
		dur, err := parseDuration(bypassDuration)
		if err != nil {
			return errors.Wrapf(err, "invalid duration %q", bypassDuration)
		}

		expiresAt = time.Now().UTC().Add(dur).Format(time.RFC3339)
	}

	cfg, err := loadScopedConfig(bypassGlobal)
	if err != nil {
		return err
	}

	skip := true
	bypassCfg := cfg.GetBypassPermissions()
	bypassCfg.SkipValidation = &skip
	bypassCfg.Reason = bypassReason
	bypassCfg.SkippedAt = time.Now().UTC().Format(time.RFC3339)
	bypassCfg.ExpiresAt = expiresAt
	bypassCfg.SkippedBy = "cli"

	if err := writeScopedConfig(cfg, bypassGlobal); err != nil {
		return err
	}

	fmt.Println("Validation SKIPPED while approval prompts are off.")

	if bypassReason != "" {
		fmt.Printf("  Reason: %s\n", bypassReason)
	}

	if expiresAt != "" {
		fmt.Printf("  Expires: %s\n", expiresAt)
	}

	fmt.Printf("\nWritten to %s config. Undo with: klaudiush bypass enforce%s\n",
		scopeName(bypassGlobal), globalFlagSuffix(bypassGlobal))

	return nil
}

func runBypassEnforce(_ *cobra.Command, _ []string) error {
	cfg, err := loadScopedConfig(bypassGlobal)
	if err != nil {
		return err
	}

	bypassCfg := cfg.GetBypassPermissions()
	bypassCfg.SkipValidation = nil
	bypassCfg.Reason = ""
	bypassCfg.SkippedAt = ""
	bypassCfg.ExpiresAt = ""
	bypassCfg.SkippedBy = ""

	pruneBypassConfig(cfg)

	if err := writeScopedConfig(cfg, bypassGlobal); err != nil {
		return err
	}

	fmt.Println("Validation ENFORCED, including while approval prompts are off.")
	fmt.Printf("\nWritten to %s config.\n", scopeName(bypassGlobal))

	return nil
}

func runBypassNotify(_ *cobra.Command, args []string) error {
	notify, err := parseOnOff(args[0])
	if err != nil {
		return err
	}

	cfg, err := loadScopedConfig(bypassGlobal)
	if err != nil {
		return err
	}

	bypassCfg := cfg.GetBypassPermissions()

	if notify {
		// Notifying is the default, so drop the entry instead of storing true.
		bypassCfg.Notify = nil
	} else {
		bypassCfg.Notify = &notify
	}

	pruneBypassConfig(cfg)

	if err := writeScopedConfig(cfg, bypassGlobal); err != nil {
		return err
	}

	fmt.Printf("Bypass reminder %s.\n", onOffLabel(notify))
	fmt.Printf("\nWritten to %s config.\n", scopeName(bypassGlobal))

	return nil
}

func runBypassStatus(_ *cobra.Command, _ []string) error {
	showProject := !bypassGlobal || bypassAll
	showGlobal := bypassGlobal || bypassAll

	if showProject {
		cfg, err := loadScopedConfig(false)
		if err != nil {
			return err
		}

		displayBypassScope(scopeProject, cfg.BypassPermissions)
	}

	if showGlobal {
		if showProject {
			fmt.Println("")
		}

		cfg, err := loadScopedConfig(true)
		if err != nil {
			return err
		}

		displayBypassScope(scopeGlobal, cfg.BypassPermissions)
	}

	return displayBypassEffective()
}

// displayBypassScope renders the bypass settings stored in one config file.
func displayBypassScope(scope string, cfg *config.BypassPermissionsConfig) {
	fmt.Printf("Bypass permissions (%s)\n", scope)

	const headerPadding = 23
	fmt.Println(strings.Repeat("=", len(scope)+headerPadding))
	fmt.Println("")

	if cfg == nil {
		fmt.Println("  (not configured)")

		return
	}

	fmt.Printf("  Validation: %s\n", validationLabel(cfg))
	fmt.Printf("  Reminder: %s\n", onOffLabel(cfg.IsNotifyEnabled()))

	if cfg.Reason != "" {
		fmt.Printf("  Reason: %s\n", cfg.Reason)
	}

	if cfg.SkippedAt != "" {
		fmt.Printf("  Since: %s\n", cfg.SkippedAt)
	}

	if cfg.ExpiresAt != "" {
		fmt.Printf("  Expires: %s\n", cfg.ExpiresAt)
	}

	if len(cfg.Modes) > 0 {
		fmt.Printf("  Modes: %s\n", strings.Join(cfg.Modes, ", "))
	}
}

// displayBypassEffective renders the merged setting the hook actually applies.
func displayBypassEffective() error {
	loader, err := internalconfig.NewKoanfLoader()
	if err != nil {
		return errors.Wrap(err, "creating config loader")
	}

	cfg, err := loader.Load(nil)
	if err != nil {
		return errors.Wrap(err, "loading merged config")
	}

	bypassCfg := cfg.BypassPermissions

	modes := bypassCfg.GetModes()
	if len(modes) == 0 {
		modes = hook.DefaultBypassModes()
	}

	fmt.Println("")
	fmt.Println("Effective")
	fmt.Println("=========")
	fmt.Println("")
	fmt.Printf("  Validation: %s\n", validationLabel(bypassCfg))
	fmt.Printf("  Reminder: %s\n", onOffLabel(bypassCfg.IsNotifyEnabled()))
	fmt.Printf("  Bypass modes: %s\n", strings.Join(modes, ", "))

	return nil
}

// validationLabel describes what happens in bypass permission modes.
func validationLabel(cfg *config.BypassPermissionsConfig) string {
	if cfg.IsSkipValidation() {
		return "SKIPPED in bypass permission modes"
	}

	if cfg != nil && cfg.IsExpired() {
		return "ENFORCED (skip expired)"
	}

	return "ENFORCED in every permission mode"
}

// pruneBypassConfig drops the bypass section when nothing is left to store.
func pruneBypassConfig(cfg *config.Config) {
	bypassCfg := cfg.BypassPermissions
	if bypassCfg == nil {
		return
	}

	if bypassCfg.SkipValidation == nil &&
		bypassCfg.Notify == nil &&
		len(bypassCfg.Modes) == 0 &&
		bypassCfg.Reason == "" &&
		bypassCfg.SkippedAt == "" &&
		bypassCfg.ExpiresAt == "" &&
		bypassCfg.SkippedBy == "" {
		cfg.BypassPermissions = nil
	}
}

// parseOnOff converts an on/off argument to a boolean.
func parseOnOff(value string) (bool, error) {
	switch strings.ToLower(value) {
	case valueOn, valueTrue, "enable", "enabled":
		return true, nil
	case valueOff, "false", "disable", "disabled":
		return false, nil
	default:
		return false, errors.Errorf("expected \"on\" or \"off\", got %q", value)
	}
}

// onOffLabel renders a boolean as on/off.
func onOffLabel(value bool) string {
	if value {
		return valueOn
	}

	return valueOff
}

// scopeName renders the config scope being written.
func scopeName(global bool) string {
	if global {
		return scopeGlobal
	}

	return scopeProject
}

// globalFlagSuffix appends --global to a suggested command when relevant.
func globalFlagSuffix(global bool) string {
	if global {
		return " --global"
	}

	return ""
}

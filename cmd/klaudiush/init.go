// Package main provides the CLI entry point for klaudiush.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/smykla-labs/klaudiush/internal/config"
	"github.com/smykla-labs/klaudiush/internal/git"
	"github.com/smykla-labs/klaudiush/internal/initcmd"
	"github.com/smykla-labs/klaudiush/internal/prompt"
	pkgConfig "github.com/smykla-labs/klaudiush/pkg/config"
)

var (
	globalFlag bool
	forceFlag  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize klaudiush configuration",
	Long: `Initialize klaudiush configuration file.

By default, creates a project-local configuration file (.klaudiush/config.toml).
Use --global or -g to create a global configuration file (~/.klaudiush/config.toml).

The initialization process will prompt you to configure:
- Git commit signoff (default: from git config user.name and user.email)
- Whether to add the config file to .git/info/exclude (project-local only)

Use --force to overwrite an existing configuration file.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().BoolVarP(
		&globalFlag,
		"global",
		"g",
		false,
		"Initialize global configuration",
	)

	initCmd.Flags().BoolVarP(
		&forceFlag,
		"force",
		"f",
		false,
		"Overwrite existing configuration file",
	)
}

func runInit(_ *cobra.Command, _ []string) error {
	prompter := prompt.NewStdPrompter()
	writer := config.NewWriter()

	// Check if config already exists
	configPath, err := checkExistingConfig(writer)
	if err != nil {
		return err
	}

	// Display initialization message
	displayInitHeader()

	// Create and configure
	cfg, err := promptConfigOptions(prompter)
	if err != nil {
		return err
	}

	// Write configuration
	if err := writeConfig(writer, cfg, configPath); err != nil {
		return err
	}

	// Handle .git/info/exclude for project config
	if err := handleGitExclude(prompter); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Configuration initialized successfully!")

	return nil
}

// checkExistingConfig checks if config already exists and returns the path.
func checkExistingConfig(writer *config.Writer) (string, error) {
	var (
		configPath   string
		configExists bool
	)

	if globalFlag {
		configPath = writer.GlobalConfigPath()
		configExists = writer.IsGlobalConfigExists()
	} else {
		configPath = writer.ProjectConfigPath()
		configExists = writer.IsProjectConfigExists()
	}

	if configExists && !forceFlag {
		return "", errors.Errorf(
			"configuration file already exists: %s\nUse --force to overwrite",
			configPath,
		)
	}

	return configPath, nil
}

// displayInitHeader displays the initialization header.
func displayInitHeader() {
	fmt.Println("╔═══════════════════════════════════════════════╗")

	if globalFlag {
		fmt.Println("║   Klaudiush Global Configuration Setup       ║")
	} else {
		fmt.Println("║   Klaudiush Project Configuration Setup      ║")
	}

	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Println()
}

// promptConfigOptions prompts for all available config options.
func promptConfigOptions(prompter prompt.Prompter) (*pkgConfig.Config, error) {
	cfg := &pkgConfig.Config{}

	// Get all available configuration options
	options := initcmd.GetDefaultOptions()

	// Filter available options
	var availableOptions []initcmd.ConfigOption

	for _, opt := range options {
		if opt.IsAvailable() {
			availableOptions = append(availableOptions, opt)
		}
	}

	// Prompt for each configuration option
	for i, opt := range availableOptions {
		if err := opt.Prompt(prompter, cfg); err != nil {
			return nil, errors.Wrapf(err, "failed to configure %s", opt.Name())
		}

		// Add separator between options (except after last)
		if i < len(availableOptions)-1 {
			fmt.Println()
		}
	}

	return cfg, nil
}

// writeConfig writes the configuration to the appropriate location.
func writeConfig(writer *config.Writer, cfg *pkgConfig.Config, configPath string) error {
	if globalFlag {
		if err := writer.WriteGlobal(cfg); err != nil {
			return errors.Wrap(err, "failed to write global configuration")
		}
	} else {
		if err := writer.WriteProject(cfg); err != nil {
			return errors.Wrap(err, "failed to write project configuration")
		}
	}

	fmt.Printf("\n✅ Configuration written to: %s\n", configPath)

	return nil
}

// handleGitExclude handles adding config to .git/info/exclude for project config.
func handleGitExclude(prompter prompt.Prompter) error {
	if globalFlag || !git.IsInGitRepo() {
		return nil
	}

	fmt.Println()

	addToExclude, err := prompter.Confirm(
		"Add config file to .git/info/exclude?",
		true,
	)
	if err != nil {
		return errors.Wrap(err, "failed to read confirmation")
	}

	if !addToExclude {
		return nil
	}

	if err := addConfigToExclude(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"⚠️  Warning: failed to add to .git/info/exclude: %v\n",
			err,
		)
	} else {
		fmt.Println("✅ Added to .git/info/exclude")
	}

	return nil
}

// addConfigToExclude adds the config file pattern to .git/info/exclude.
func addConfigToExclude() error {
	excludeMgr, err := git.NewExcludeManager()
	if err != nil {
		return errors.Wrap(err, "failed to create exclude manager")
	}

	// Add both config file patterns
	patterns := []string{
		filepath.Join(config.ProjectConfigDir, config.ProjectConfigFile),
		config.ProjectConfigFileAlt,
	}

	for _, pattern := range patterns {
		if err := excludeMgr.AddEntry(pattern); err != nil {
			if !errors.Is(err, git.ErrEntryAlreadyExists) {
				return errors.Wrapf(err, "failed to add %s", pattern)
			}
		}
	}

	return nil
}

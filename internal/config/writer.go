// Package config provides internal configuration loading and processing.
package config

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/pkg/errors"

	"github.com/smykla-labs/klaudiush/pkg/config"
)

const (
	// ConfigFileMode is the file mode for configuration files (user read/write only).
	ConfigFileMode = 0o600

	// ConfigDirMode is the file mode for configuration directories (user rwx only).
	ConfigDirMode = 0o700
)

// Writer handles writing configuration to TOML files.
type Writer struct {
	// homeDir is the user's home directory (for testing).
	homeDir string

	// workDir is the current working directory (for testing).
	workDir string
}

// NewWriter creates a new Writer with default directories.
func NewWriter() *Writer {
	return &Writer{
		homeDir: os.Getenv("HOME"),
		workDir: mustGetwd(),
	}
}

// NewWriterWithDirs creates a new Writer with custom directories (for testing).
func NewWriterWithDirs(homeDir, workDir string) *Writer {
	return &Writer{
		homeDir: homeDir,
		workDir: workDir,
	}
}

// WriteGlobal writes the configuration to the global config file.
func (w *Writer) WriteGlobal(cfg *config.Config) error {
	path := w.GlobalConfigPath()

	return w.WriteFile(path, cfg)
}

// WriteProject writes the configuration to the project config file.
// Uses the primary location (.klaudiush/config.toml).
func (w *Writer) WriteProject(cfg *config.Config) error {
	path := w.ProjectConfigPath()

	return w.WriteFile(path, cfg)
}

// WriteFile writes the configuration to the given path.
func (*Writer) WriteFile(path string, cfg *config.Config) error {
	if cfg == nil {
		return errors.Wrap(ErrInvalidConfig, "config is nil")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, ConfigDirMode); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	// Marshal to TOML with indentation
	var buf bytes.Buffer

	encoder := toml.NewEncoder(&buf)
	encoder.SetIndentTables(true)

	if err := encoder.Encode(cfg); err != nil {
		return errors.Wrap(err, "failed to encode config to TOML")
	}

	// Write to file with secure permissions
	if err := os.WriteFile(path, buf.Bytes(), ConfigFileMode); err != nil {
		return errors.Wrapf(err, "failed to write config file %s", path)
	}

	return nil
}

// GlobalConfigPath returns the path to the global configuration file.
func (w *Writer) GlobalConfigPath() string {
	return filepath.Join(w.homeDir, GlobalConfigDir, GlobalConfigFile)
}

// ProjectConfigPath returns the path to the primary project configuration file.
func (w *Writer) ProjectConfigPath() string {
	return filepath.Join(w.workDir, ProjectConfigDir, ProjectConfigFile)
}

// EnsureGlobalConfigDir ensures the global config directory exists.
func (w *Writer) EnsureGlobalConfigDir() error {
	dir := filepath.Join(w.homeDir, GlobalConfigDir)

	if err := os.MkdirAll(dir, ConfigDirMode); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	return nil
}

// EnsureProjectConfigDir ensures the project config directory exists.
func (w *Writer) EnsureProjectConfigDir() error {
	dir := filepath.Join(w.workDir, ProjectConfigDir)

	if err := os.MkdirAll(dir, ConfigDirMode); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	return nil
}

// IsGlobalConfigExists checks if the global config file exists.
func (w *Writer) IsGlobalConfigExists() bool {
	path := w.GlobalConfigPath()
	_, err := os.Stat(path)

	return err == nil
}

// IsProjectConfigExists checks if the project config file exists.
func (w *Writer) IsProjectConfigExists() bool {
	path := w.ProjectConfigPath()
	_, err := os.Stat(path)

	return err == nil
}

// GlobalConfigDir returns the global config directory path.
func (w *Writer) GlobalConfigDir() string {
	return filepath.Join(w.homeDir, GlobalConfigDir)
}

// ProjectConfigDir returns the project config directory path.
func (w *Writer) ProjectConfigDir() string {
	return filepath.Join(w.workDir, ProjectConfigDir)
}

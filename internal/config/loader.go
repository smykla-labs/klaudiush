// Package config provides internal configuration loading and processing.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/smykla-labs/klaudiush/pkg/config"
)

var (
	// ErrConfigNotFound is returned when no configuration file is found.
	ErrConfigNotFound = errors.New("configuration file not found")

	// ErrInvalidTOML is returned when the TOML file cannot be parsed.
	ErrInvalidTOML = errors.New("invalid TOML")

	// ErrInvalidPermissions is returned when config file has insecure permissions.
	ErrInvalidPermissions = errors.New("config file has insecure permissions")
)

const (
	// GlobalConfigFile is the name of the global configuration file.
	GlobalConfigFile = "config.toml"

	// GlobalConfigDir is the directory name for global configuration.
	GlobalConfigDir = ".klaudiush"

	// ProjectConfigDir is the directory name for project configuration.
	ProjectConfigDir = ".klaudiush"

	// ProjectConfigFile is the primary project configuration file name.
	ProjectConfigFile = "config.toml"

	// ProjectConfigFileAlt is the alternative project configuration file name.
	ProjectConfigFileAlt = "klaudiush.toml"
)

// Loader handles loading configuration from TOML files.
type Loader struct {
	// homeDir is the user's home directory (for testing).
	homeDir string

	// workDir is the current working directory (for testing).
	workDir string
}

// NewLoader creates a new Loader with default directories.
func NewLoader() *Loader {
	return &Loader{
		homeDir: os.Getenv("HOME"),
		workDir: mustGetwd(),
	}
}

// NewLoaderWithDirs creates a new Loader with custom directories (for testing).
func NewLoaderWithDirs(homeDir, workDir string) *Loader {
	return &Loader{
		homeDir: homeDir,
		workDir: workDir,
	}
}

// LoadGlobal loads the global configuration file from ~/.klaudiush/config.toml.
// Returns ErrConfigNotFound if the file doesn't exist.
func (l *Loader) LoadGlobal() (*config.Config, error) {
	path := l.GlobalConfigPath()

	return l.LoadFile(path)
}

// LoadProject loads the project configuration file.
// Checks .klaudiush/config.toml first, then klaudiush.toml.
// Returns ErrConfigNotFound if no file is found.
func (l *Loader) LoadProject() (*config.Config, error) {
	// Try primary location first
	primaryPath := filepath.Join(l.workDir, ProjectConfigDir, ProjectConfigFile)

	cfg, err := l.LoadFile(primaryPath)
	if err == nil {
		return cfg, nil
	}

	if !errors.Is(err, ErrConfigNotFound) {
		return nil, err
	}

	// Try alternative location
	altPath := filepath.Join(l.workDir, ProjectConfigFileAlt)

	return l.LoadFile(altPath)
}

// LoadFile loads a configuration file from the given path.
// Returns ErrConfigNotFound if the file doesn't exist.
func (*Loader) LoadFile(path string) (*config.Config, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}

		return nil, fmt.Errorf("failed to stat config file %s: %w", path, err)
	}

	// Warn on world-writable permissions (security check)
	if info.Mode().Perm()&0o002 != 0 {
		return nil, fmt.Errorf(
			"%w: %s is world-writable (mode: %s)",
			ErrInvalidPermissions,
			path,
			info.Mode().Perm(),
		)
	}

	// Read file
	//nolint:gosec // G304: File path comes from known config locations
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Parse TOML with strict mode
	var cfg config.Config

	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("%w in %s: %w", ErrInvalidTOML, path, err)
	}

	return &cfg, nil
}

// GlobalConfigPath returns the path to the global configuration file.
func (l *Loader) GlobalConfigPath() string {
	return filepath.Join(l.homeDir, GlobalConfigDir, GlobalConfigFile)
}

// ProjectConfigPaths returns the paths to check for project configuration.
// Returns paths in order of precedence.
func (l *Loader) ProjectConfigPaths() []string {
	return []string{
		filepath.Join(l.workDir, ProjectConfigDir, ProjectConfigFile),
		filepath.Join(l.workDir, ProjectConfigFileAlt),
	}
}

// HasGlobalConfig checks if a global configuration file exists.
func (l *Loader) HasGlobalConfig() bool {
	path := l.GlobalConfigPath()
	_, err := os.Stat(path)

	return err == nil
}

// HasProjectConfig checks if a project configuration file exists.
func (l *Loader) HasProjectConfig() bool {
	for _, path := range l.ProjectConfigPaths() {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// mustGetwd returns the current working directory or panics.
func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("failed to get working directory: %s", err))
	}

	return wd
}

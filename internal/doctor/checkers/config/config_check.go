// Package configchecker provides checkers for configuration file validation.
package configchecker

import (
	"context"
	"errors"
	"fmt"

	internalconfig "github.com/smykla-labs/klaudiush/internal/config"
	"github.com/smykla-labs/klaudiush/internal/doctor"
	"github.com/smykla-labs/klaudiush/pkg/config"
)

// ConfigLoader defines the interface for configuration loading operations.
//
//go:generate mockgen -source=config_check.go -destination=config_loader_mock.go -package=configchecker
type ConfigLoader interface {
	HasGlobalConfig() bool
	HasProjectConfig() bool
	GlobalConfigPath() string
	Load(flags map[string]any) (*config.Config, error)
}

// GlobalChecker checks the validity of the global configuration
type GlobalChecker struct {
	loader ConfigLoader
}

// NewGlobalChecker creates a new global config checker
func NewGlobalChecker() *GlobalChecker {
	loader, _ := internalconfig.NewKoanfLoader()

	return &GlobalChecker{
		loader: loader,
	}
}

// NewGlobalCheckerWithLoader creates a GlobalChecker with a custom loader (for testing).
func NewGlobalCheckerWithLoader(loader ConfigLoader) *GlobalChecker {
	return &GlobalChecker{
		loader: loader,
	}
}

// Name returns the name of the check
func (*GlobalChecker) Name() string {
	return "Global config"
}

// Category returns the category of the check
func (*GlobalChecker) Category() doctor.Category {
	return doctor.CategoryConfig
}

// Check performs the global config validity check
func (c *GlobalChecker) Check(_ context.Context) doctor.CheckResult {
	if !c.loader.HasGlobalConfig() {
		return doctor.FailWarning("Global config", "Not found (optional)").
			WithDetails(
				"Expected at: "+c.loader.GlobalConfigPath(),
				"Create with: klaudiush init --global",
			).
			WithFixID("create_global_config")
	}

	// Try loading config to validate it
	cfg, err := c.loader.Load(nil)
	if err != nil {
		if errors.Is(err, internalconfig.ErrInvalidTOML) {
			return doctor.FailError("Global config", "Invalid TOML syntax").
				WithDetails(
					"File: "+c.loader.GlobalConfigPath(),
					fmt.Sprintf("Error: %v", err),
				)
		}

		if errors.Is(err, internalconfig.ErrInvalidPermissions) {
			return doctor.FailError("Global config", "Insecure file permissions").
				WithDetails(
					"File: "+c.loader.GlobalConfigPath(),
					"Config file should not be world-writable",
					"Fix with: chmod 600 <config-file>",
				).
				WithFixID("fix_config_permissions")
		}

		return doctor.FailError("Global config", fmt.Sprintf("Failed to load: %v", err))
	}

	// Validate config semantics
	validator := internalconfig.NewValidator()
	if err := validator.Validate(cfg); err != nil {
		return doctor.FailError("Global config", "Validation failed").
			WithDetails(
				"File: "+c.loader.GlobalConfigPath(),
				fmt.Sprintf("Error: %v", err),
			)
	}

	return doctor.Pass("Global config", "Loaded and validated")
}

// ProjectChecker checks the validity of the project configuration
type ProjectChecker struct {
	loader ConfigLoader
}

// NewProjectChecker creates a new project config checker
func NewProjectChecker() *ProjectChecker {
	loader, _ := internalconfig.NewKoanfLoader()

	return &ProjectChecker{
		loader: loader,
	}
}

// NewProjectCheckerWithLoader creates a ProjectChecker with a custom loader (for testing).
func NewProjectCheckerWithLoader(loader ConfigLoader) *ProjectChecker {
	return &ProjectChecker{
		loader: loader,
	}
}

// Name returns the name of the check
func (*ProjectChecker) Name() string {
	return "Project config"
}

// Category returns the category of the check
func (*ProjectChecker) Category() doctor.Category {
	return doctor.CategoryConfig
}

// Check performs the project config validity check
func (c *ProjectChecker) Check(_ context.Context) doctor.CheckResult {
	if !c.loader.HasProjectConfig() {
		// Project config not found is just informational since global config is the primary
		return doctor.Skip("Project config", "Not found (using global config)")
	}

	cfg, err := c.loader.Load(nil)
	if err != nil {
		if errors.Is(err, internalconfig.ErrInvalidTOML) {
			return doctor.FailError("Project config", "Invalid TOML syntax").
				WithDetails(fmt.Sprintf("Error: %v", err))
		}

		if errors.Is(err, internalconfig.ErrInvalidPermissions) {
			return doctor.FailError("Project config", "Insecure file permissions").
				WithDetails(
					"Config file should not be world-writable",
					"Fix with: chmod 600 <config-file>",
				).
				WithFixID("fix_config_permissions")
		}

		return doctor.FailError("Project config", fmt.Sprintf("Failed to load: %v", err))
	}

	// Validate config semantics
	validator := internalconfig.NewValidator()
	if err := validator.Validate(cfg); err != nil {
		return doctor.FailError("Project config", "Validation failed").
			WithDetails(fmt.Sprintf("Error: %v", err))
	}

	return doctor.Pass("Project config", "Loaded and validated")
}

// PermissionsChecker checks if config files have secure permissions
type PermissionsChecker struct {
	loader ConfigLoader
}

// NewPermissionsChecker creates a new permissions checker
func NewPermissionsChecker() *PermissionsChecker {
	loader, _ := internalconfig.NewKoanfLoader()

	return &PermissionsChecker{
		loader: loader,
	}
}

// NewPermissionsCheckerWithLoader creates a PermissionsChecker with a custom loader (for testing).
func NewPermissionsCheckerWithLoader(loader ConfigLoader) *PermissionsChecker {
	return &PermissionsChecker{
		loader: loader,
	}
}

// Name returns the name of the check
func (*PermissionsChecker) Name() string {
	return "Config permissions"
}

// Category returns the category of the check
func (*PermissionsChecker) Category() doctor.Category {
	return doctor.CategoryConfig
}

// Check performs the permissions check
func (c *PermissionsChecker) Check(_ context.Context) doctor.CheckResult {
	// Check both global and project configs
	hasGlobal := c.loader.HasGlobalConfig()
	hasProject := c.loader.HasProjectConfig()

	if !hasGlobal && !hasProject {
		return doctor.Skip("Config permissions", "No config files found")
	}

	// Try loading - if they have permission issues, they'll fail
	_, err := c.loader.Load(nil)

	// Check for permission errors
	if err != nil && errors.Is(err, internalconfig.ErrInvalidPermissions) {
		return doctor.FailError("Config permissions", "Insecure file permissions detected").
			WithDetails(
				fmt.Sprintf("Error: %v", err),
				"Fix with: chmod 600 <config-file>",
			).
			WithFixID("fix_config_permissions")
	}

	return doctor.Pass("Config permissions", "Files are secured")
}

package file

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	execpkg "github.com/smykla-labs/claude-hooks/internal/exec"
	"github.com/smykla-labs/claude-hooks/internal/validator"
	"github.com/smykla-labs/claude-hooks/pkg/hook"
	"github.com/smykla-labs/claude-hooks/pkg/logger"
)

const (
	// terraformTimeout is the timeout for terraform/tofu commands
	terraformTimeout = 10 * time.Second
)

// TerraformValidator validates Terraform/OpenTofu file formatting
type TerraformValidator struct {
	validator.BaseValidator
	toolChecker execpkg.ToolChecker
	runner      execpkg.CommandRunner
	tempManager execpkg.TempFileManager
}

// NewTerraformValidator creates a new TerraformValidator
func NewTerraformValidator(log logger.Logger) *TerraformValidator {
	return &TerraformValidator{
		BaseValidator: *validator.NewBaseValidator("validate-terraform", log),
		toolChecker:   execpkg.NewToolChecker(),
		runner:        execpkg.NewCommandRunner(terraformTimeout),
		tempManager:   execpkg.NewTempFileManager(),
	}
}

// Validate checks Terraform formatting and optionally runs tflint
func (v *TerraformValidator) Validate(ctx *hook.Context) *validator.Result {
	log := v.Logger()
	content, err := v.getContent(ctx)
	if err != nil {
		log.Debug("skipping terraform validation", "error", err)
		return validator.Pass()
	}

	if content == "" {
		return validator.Pass()
	}

	// Detect which tool to use
	tool := v.detectTool()
	log.Debug("detected terraform tool", "tool", tool)

	// Create temp file for validation
	tmpFile, cleanup, err := v.tempManager.Create("terraform-*.tf", content)
	if err != nil {
		log.Debug("failed to create temp file", "error", err)
		return validator.Pass()
	}
	defer cleanup()

	var warnings []string

	// Run format check
	if fmtWarning := v.checkFormat(tool, tmpFile); fmtWarning != "" {
		warnings = append(warnings, fmtWarning)
	}

	// Run tflint if available
	if lintWarnings := v.runTflint(tmpFile); len(lintWarnings) > 0 {
		warnings = append(warnings, lintWarnings...)
	}

	if len(warnings) > 0 {
		message := "Terraform validation warnings"
		details := map[string]string{
			"warnings": strings.Join(warnings, "\n"),
		}
		return validator.WarnWithDetails(message, details)
	}

	return validator.Pass()
}

// getContent extracts terraform content from context
func (v *TerraformValidator) getContent(ctx *hook.Context) (string, error) {
	// Try to get content from tool input (Write operation)
	if ctx.ToolInput.Content != "" {
		return ctx.ToolInput.Content, nil
	}

	// For Edit operations in PreToolUse, we can't easily get final content
	// Skip validation
	if ctx.EventType == hook.PreToolUse && ctx.ToolName == hook.Edit {
		return "", errors.New("cannot validate Edit operations in PreToolUse")
	}

	// Try to get from file path (Edit or PostToolUse)
	filePath := ctx.GetFilePath()
	if filePath != "" {
		// In PostToolUse, we could read the file, but for now skip
		// as the Bash version doesn't handle this case well either
		return "", errors.New("file-based validation not implemented")
	}

	return "", errors.New("no content found")
}

// detectTool detects whether to use tofu or terraform
func (v *TerraformValidator) detectTool() string {
	return v.toolChecker.FindTool("tofu", "terraform")
}

// checkFormat runs terraform/tofu fmt -check
func (v *TerraformValidator) checkFormat(tool, filePath string) string {
	if tool == "" {
		return "⚠️  Neither 'tofu' nor 'terraform' found in PATH - skipping format check"
	}

	ctx, cancel := context.WithTimeout(context.Background(), terraformTimeout)
	defer cancel()

	result, err := v.runner.Run(ctx, tool, "fmt", "-check", "-diff", filePath)
	if err == nil {
		// Formatting is correct
		return ""
	}

	// Format check failed - terraform fmt returns exit 3 when formatting is needed
	diff := result.Stdout
	if diff == "" {
		diff = result.Stderr
	}

	if strings.TrimSpace(diff) != "" {
		return fmt.Sprintf("⚠️  Terraform formatting issues detected:\n%s\n   Run '%s fmt %s' to fix",
			strings.TrimSpace(diff), tool, filepath.Base(filePath))
	}

	v.Logger().Debug("fmt command failed", "error", err, "stderr", result.Stderr)
	return fmt.Sprintf("⚠️  Failed to run '%s fmt -check': %v", tool, err)
}

// runTflint runs tflint on the file if available
func (v *TerraformValidator) runTflint(filePath string) []string {
	// Check if tflint is available
	if !v.toolChecker.IsAvailable("tflint") {
		v.Logger().Debug("tflint not found in PATH, skipping")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), terraformTimeout)
	defer cancel()

	// Run tflint on the file
	result, err := v.runner.Run(ctx, "tflint", "--format=compact", filePath)
	output := strings.TrimSpace(result.Stdout)

	if err != nil {
		// tflint returns non-zero on findings
		if output != "" {
			return []string{"⚠️  tflint findings:\n" + output}
		}
		v.Logger().Debug("tflint failed", "error", err, "stderr", result.Stderr)
		return nil
	}

	// No findings
	return nil
}

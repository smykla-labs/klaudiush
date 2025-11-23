package linters

import (
	"context"

	execpkg "github.com/smykla-labs/claude-hooks/internal/exec"
)

// TerraformFormatter validates and formats Terraform/OpenTofu files
type TerraformFormatter interface {
	CheckFormat(ctx context.Context, content string) *LintResult
	DetectTool() string
}

// RealTerraformFormatter implements TerraformFormatter using terraform/tofu CLI
type RealTerraformFormatter struct {
	runner      execpkg.CommandRunner
	toolChecker execpkg.ToolChecker
	tempManager execpkg.TempFileManager
}

// NewTerraformFormatter creates a new RealTerraformFormatter
func NewTerraformFormatter(runner execpkg.CommandRunner) *RealTerraformFormatter {
	return &RealTerraformFormatter{
		runner:      runner,
		toolChecker: execpkg.NewToolChecker(),
		tempManager: execpkg.NewTempFileManager(),
	}
}

// DetectTool detects whether to use tofu or terraform
func (t *RealTerraformFormatter) DetectTool() string {
	return t.toolChecker.FindTool("tofu", "terraform")
}

// CheckFormat validates Terraform file formatting
func (t *RealTerraformFormatter) CheckFormat(ctx context.Context, content string) *LintResult {
	tool := t.DetectTool()
	if tool == "" {
		return &LintResult{
			Success: true,
			Err:     nil,
		}
	}

	// Create temp file for validation
	tmpFile, cleanup, err := t.tempManager.Create("terraform-*.tf", content)
	if err != nil {
		return &LintResult{
			Success: false,
			Err:     err,
		}
	}
	defer cleanup()

	// Run terraform fmt -check -diff
	result, err := t.runner.Run(ctx, tool, "fmt", "-check", "-diff", tmpFile)

	return &LintResult{
		Success:  err == nil,
		RawOut:   result.Stdout + result.Stderr,
		Findings: []LintFinding{}, // TODO: Parse diff output
		Err:      err,
	}
}

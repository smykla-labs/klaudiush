package linters

import (
	"context"

	execpkg "github.com/smykla-labs/claude-hooks/internal/exec"
)

// TfLinter validates Terraform files using tflint
type TfLinter interface {
	Lint(ctx context.Context, filePath string) *LintResult
}

// RealTfLinter implements TfLinter using the tflint CLI tool
type RealTfLinter struct {
	runner      execpkg.CommandRunner
	toolChecker execpkg.ToolChecker
}

// NewTfLinter creates a new RealTfLinter
func NewTfLinter(runner execpkg.CommandRunner) *RealTfLinter {
	return &RealTfLinter{
		runner:      runner,
		toolChecker: execpkg.NewToolChecker(),
	}
}

// Lint validates Terraform file using tflint
func (t *RealTfLinter) Lint(ctx context.Context, filePath string) *LintResult {
	// Check if tflint is available
	if !t.toolChecker.IsAvailable("tflint") {
		return &LintResult{
			Success: true,
			Err:     nil,
		}
	}

	// Run tflint with compact format
	result := t.runner.Run(ctx, "tflint", "--format=compact", filePath)
	// tflint returns non-zero when findings are detected
	if result.Err != nil {
		// If there's output, it means there are findings (not an error)
		output := result.Stdout
		if output == "" {
			output = result.Stderr
		}

		if output != "" {
			return &LintResult{
				Success:  false,
				RawOut:   output,
				Findings: []LintFinding{}, // TODO: Parse compact output
				Err:      result.Err,
			}
		}

		// Real error
		return &LintResult{
			Success: false,
			Err:     result.Err,
		}
	}

	// No findings
	return &LintResult{
		Success:  true,
		RawOut:   result.Stdout,
		Findings: []LintFinding{},
		Err:      nil,
	}
}

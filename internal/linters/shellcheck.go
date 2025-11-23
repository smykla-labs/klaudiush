package linters

import (
	"context"

	execpkg "github.com/smykla-labs/claude-hooks/internal/exec"
)

// ShellChecker validates shell scripts using shellcheck
type ShellChecker interface {
	Check(ctx context.Context, content string) *LintResult
}

// RealShellChecker implements ShellChecker using the shellcheck CLI tool
type RealShellChecker struct {
	runner      execpkg.CommandRunner
	toolChecker execpkg.ToolChecker
	tempManager execpkg.TempFileManager
}

// NewShellChecker creates a new RealShellChecker
func NewShellChecker(runner execpkg.CommandRunner) *RealShellChecker {
	return &RealShellChecker{
		runner:      runner,
		toolChecker: execpkg.NewToolChecker(),
		tempManager: execpkg.NewTempFileManager(),
	}
}

// Check validates shell script content using shellcheck
func (s *RealShellChecker) Check(ctx context.Context, content string) *LintResult {
	// Check if shellcheck is available
	if !s.toolChecker.IsAvailable("shellcheck") {
		return &LintResult{
			Success: true,
			Err:     nil,
		}
	}

	// Create temp file for validation
	tmpFile, cleanup, err := s.tempManager.Create("script-*.sh", content)
	if err != nil {
		return &LintResult{
			Success: false,
			Err:     err,
		}
	}
	defer cleanup()

	// Run shellcheck
	result := s.runner.Run(ctx, "shellcheck", tmpFile)

	return &LintResult{
		Success:  result.Err == nil,
		RawOut:   result.Stdout + result.Stderr,
		Findings: []LintFinding{}, // TODO: Parse shellcheck output
		Err:      result.Err,
	}
}

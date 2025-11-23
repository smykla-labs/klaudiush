package linters

import (
	"context"
	"strings"

	execpkg "github.com/smykla-labs/claude-hooks/internal/exec"
)

// MarkdownLinter validates Markdown files using markdownlint
type MarkdownLinter interface {
	Lint(ctx context.Context, content string) *LintResult
}

// RealMarkdownLinter implements MarkdownLinter using the markdownlint CLI tool
type RealMarkdownLinter struct {
	runner      execpkg.CommandRunner
	toolChecker execpkg.ToolChecker
}

// NewMarkdownLinter creates a new RealMarkdownLinter
func NewMarkdownLinter(runner execpkg.CommandRunner) *RealMarkdownLinter {
	return &RealMarkdownLinter{
		runner:      runner,
		toolChecker: execpkg.NewToolChecker(),
	}
}

// Lint validates Markdown content
func (m *RealMarkdownLinter) Lint(ctx context.Context, content string) *LintResult {
	// Check if markdownlint is available
	if !m.toolChecker.IsAvailable("markdownlint") {
		return &LintResult{
			Success: true,
			Err:     nil,
		}
	}

	// Run markdownlint with stdin input
	result, err := m.runner.RunWithStdin(ctx, strings.NewReader(content), "markdownlint", "--stdin")

	return &LintResult{
		Success:  err == nil,
		RawOut:   result.Stdout + result.Stderr,
		Findings: []LintFinding{}, // TODO: Parse markdownlint output
		Err:      err,
	}
}

package linters

import (
	"context"
	"errors"
	"strings"

	execpkg "github.com/smykla-labs/claude-hooks/internal/exec"
	"github.com/smykla-labs/claude-hooks/internal/validators"
)

// ErrMarkdownCustomRules indicates custom markdown rules found issues
var ErrMarkdownCustomRules = errors.New("custom markdown rules validation failed")

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

// Lint validates Markdown content using markdownlint CLI and custom rules
func (m *RealMarkdownLinter) Lint(ctx context.Context, content string) *LintResult {
	var combinedOutput strings.Builder

	var combinedErr error

	if m.toolChecker.IsAvailable("markdownlint") {
		result := m.runner.RunWithStdin(
			ctx,
			strings.NewReader(content),
			"markdownlint",
			"--stdin",
		)
		if result.Stdout != "" || result.Stderr != "" {
			combinedOutput.WriteString(result.Stdout)
			combinedOutput.WriteString(result.Stderr)
		}

		if result.Err != nil {
			combinedErr = result.Err
		}
	}

	// Run custom markdown analysis
	analysisResult := validators.AnalyzeMarkdown(content)
	if len(analysisResult.Warnings) > 0 {
		if combinedOutput.Len() > 0 {
			combinedOutput.WriteString("\n")
		}

		combinedOutput.WriteString("Custom rules:\n")
		combinedOutput.WriteString(strings.Join(analysisResult.Warnings, "\n"))

		// If custom rules found issues, mark as failed
		if combinedErr == nil {
			combinedErr = ErrMarkdownCustomRules
		}
	}

	success := combinedErr == nil
	rawOut := combinedOutput.String()

	return &LintResult{
		Success:  success,
		RawOut:   rawOut,
		Findings: []LintFinding{}, // TODO: Parse markdownlint output into structured findings
		Err:      combinedErr,
	}
}

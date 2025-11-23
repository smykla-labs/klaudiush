package git

import (
	"context"
	"strings"
	"time"

	execpkg "github.com/smykla-labs/claude-hooks/internal/exec"
)

const (
	markdownlintTimeout = 5 * time.Second
)

// PRMarkdownValidationResult contains the result of markdown validation
type PRMarkdownValidationResult struct {
	Errors []string
}

// ValidatePRMarkdown runs markdownlint on the PR body content
func ValidatePRMarkdown(body string) PRMarkdownValidationResult {
	result := PRMarkdownValidationResult{
		Errors: []string{},
	}

	if body == "" || body == "<body-present-but-extraction-failed>" {
		return result
	}

	// Check if markdownlint is available
	checker := execpkg.NewToolChecker()
	if !checker.IsAvailable("markdownlint") {
		// markdownlint not installed, skip validation
		return result
	}

	// Run markdownlint with stdin input
	ctx, cancel := context.WithTimeout(context.Background(), markdownlintTimeout)
	defer cancel()

	runner := execpkg.NewCommandRunner(markdownlintTimeout)
	cmdResult := runner.RunWithStdin(ctx, strings.NewReader(body), "markdownlint", "--stdin")

	// Parse markdownlint output
	output := cmdResult.Stdout + cmdResult.Stderr
	if output == "" {
		return result
	}

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// markdownlint output format: stdin:line[:column] MD### description
		if strings.Contains(trimmed, "MD") {
			// Remove 'stdin:' prefix for cleaner output
			cleaned := strings.TrimPrefix(trimmed, "stdin:")
			result.Errors = append(result.Errors, "Markdown: "+cleaned)
		}
	}

	return result
}

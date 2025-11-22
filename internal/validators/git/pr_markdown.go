package git

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), markdownlintTimeout)
	defer cancel()

	if _, err := exec.LookPath("markdownlint"); err != nil {
		// markdownlint not installed, skip validation
		return result
	}

	// Run markdownlint with stdin input
	cmd := exec.CommandContext(ctx, "markdownlint", "--stdin")
	cmd.Stdin = strings.NewReader(body)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run() // Ignore error, we check output instead

	// Parse markdownlint output
	output := stdout.String() + stderr.String()
	if output == "" {
		return result
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
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

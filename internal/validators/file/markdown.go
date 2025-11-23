// Package file provides validators for file operations
package file

import (
	"errors"
	"strings"

	"github.com/smykla-labs/claude-hooks/internal/validator"
	"github.com/smykla-labs/claude-hooks/internal/validators"
	"github.com/smykla-labs/claude-hooks/pkg/hook"
	"github.com/smykla-labs/claude-hooks/pkg/logger"
)

var (
	errCannotValidateEdit    = errors.New("cannot validate Edit operations in PreToolUse")
	errFileValidationNotImpl = errors.New("file-based validation not implemented")
	errNoContent             = errors.New("no content found")
)

// MarkdownValidator validates Markdown formatting rules
type MarkdownValidator struct {
	validator.BaseValidator
}

// NewMarkdownValidator creates a new MarkdownValidator
func NewMarkdownValidator(log logger.Logger) *MarkdownValidator {
	return &MarkdownValidator{
		BaseValidator: *validator.NewBaseValidator("validate-markdown", log),
	}
}

// Validate checks Markdown formatting rules
func (v *MarkdownValidator) Validate(ctx *hook.Context) *validator.Result {
	log := v.Logger()

	content, err := v.getContent(ctx)
	if err != nil {
		log.Debug("skipping markdown validation", "error", err)
		return validator.Pass()
	}

	if content == "" {
		return validator.Pass()
	}

	result := validators.AnalyzeMarkdown(content)

	if len(result.Warnings) > 0 {
		message := "Markdown formatting errors"
		details := map[string]string{
			"errors": strings.Join(result.Warnings, "\n"),
		}

		return validator.FailWithDetails(message, details)
	}

	return validator.Pass()
}

// getContent extracts markdown content from context
func (v *MarkdownValidator) getContent(ctx *hook.Context) (string, error) {
	// Try to get content from tool input (Write operation)
	if ctx.ToolInput.Content != "" {
		return ctx.ToolInput.Content, nil
	}

	// For Edit operations in PreToolUse, we can't easily get final content
	// Skip validation
	if ctx.EventType == hook.PreToolUse && ctx.ToolName == hook.Edit {
		return "", errCannotValidateEdit
	}

	// Try to get from file path (Edit or PostToolUse)
	filePath := ctx.GetFilePath()
	if filePath != "" {
		// In PostToolUse, we could read the file, but for now skip
		// as the Bash version doesn't handle this case well either
		return "", errFileValidationNotImpl
	}

	return "", errNoContent
}

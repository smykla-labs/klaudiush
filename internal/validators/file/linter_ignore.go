package file

import (
	"context"
	"regexp"

	"github.com/smykla-skalski/klaudiush/internal/validator"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

// defaultIgnorePatterns are common linter ignore directives to block.
// These patterns are compiled into regexes that match ignore directives.
var defaultIgnorePatterns = []string{
	// Python
	`#\s*noqa`,              // # noqa, # noqa: E501
	`#\s*type:\s*ignore`,    // # type: ignore
	`#\s*pylint:\s*disable`, // # pylint: disable=...
	`#\s*pyright:\s*ignore`, // # pyright: ignore
	`#\s*mypy:\s*ignore`,    // mypy suppress directive
	`#\s*pyrefly:\s*ignore`, // pyrefly suppress directive

	// JavaScript/TypeScript
	`//\s*eslint-disable`,   // // eslint-disable
	`//\s*@ts-ignore`,       // // @ts-ignore
	`//\s*@ts-nocheck`,      // // @ts-nocheck
	`//\s*@ts-expect-error`, // // @ts-expect-error
	`/\*\s*eslint-disable`,  // /* eslint-disable */

	// Go
	`//nolint`,    // //nolint, //nolint:errcheck
	`//\s*nolint`, // // nolint

	// Rust
	`#\[allow\(`,  // #[allow(dead_code)]
	`#!\[allow\(`, // #![allow(missing_docs)]

	// Ruby
	`#\s*rubocop:\s*disable`, // # rubocop:disable

	// Shell
	`#\s*shellcheck\s+disable`, // # shellcheck disable=SC2086

	// Java
	`@SuppressWarnings`, // @SuppressWarnings("unchecked")

	// C#
	`#pragma\s+warning\s+disable`, // #pragma warning disable CS0618

	// PHP
	`//\s*phpcs:ignore`,    // // phpcs:ignore
	`//\s*@phpstan-ignore`, // // @phpstan-ignore

	// Swift
	`//\s*swiftlint:disable`, // // swiftlint:disable
}

// LinterIgnoreValidator validates that code does not contain linter ignore directives.
type LinterIgnoreValidator struct {
	validator.BaseValidator
	config   *config.LinterIgnoreValidatorConfig
	patterns []*regexp.Regexp
}

// NewLinterIgnoreValidator creates a new LinterIgnoreValidator.
func NewLinterIgnoreValidator(
	log logger.Logger,
	cfg *config.LinterIgnoreValidatorConfig,
	ruleAdapter validator.RuleChecker,
) *LinterIgnoreValidator {
	v := &LinterIgnoreValidator{
		BaseValidator: *validator.NewBaseValidatorWithRules("validate-linter-ignore", log, ruleAdapter),
		config:        cfg,
	}

	v.patterns = compilePatterns(log, "linter ignore", v.getPatterns())

	return v
}

// Validate checks for linter ignore directives in file content.
func (v *LinterIgnoreValidator) Validate(
	ctx context.Context,
	hookCtx *hook.Context,
) *validator.Result {
	return validatePatterns(
		ctx, hookCtx, v.CheckRules, v.patterns,
		"Linter ignore directives are not allowed", validator.RefLinterIgnore,
	)
}

// getPatterns returns the configured patterns or defaults.
func (v *LinterIgnoreValidator) getPatterns() []string {
	if v.config != nil && len(v.config.Patterns) > 0 {
		return v.config.Patterns
	}

	return defaultIgnorePatterns
}

// Category returns the validator category for parallel execution.
// LinterIgnoreValidator uses CategoryCPU because it only does pattern matching.
func (*LinterIgnoreValidator) Category() validator.ValidatorCategory {
	return validator.CategoryCPU
}

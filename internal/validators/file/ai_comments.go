package file

import (
	"context"
	"regexp"

	"github.com/smykla-skalski/klaudiush/internal/validator"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

// defaultAICommentPatterns match filler comments that only restate the line
// of code they annotate, a pattern common in LLM-generated output. Each
// pattern matches a "//" or "#" comment marker followed by a throat-clearing
// phrase; comments explaining *why* rarely open this way.
var defaultAICommentPatterns = []string{
	`(?i)(//|#)\s*initiali[sz](e|ing)\s`,
	`(?i)(//|#)\s*(loop|iterate)(s|ing)?\s+(through|over)\b`,
	`(?i)(//|#)\s*check(s|ing)?\s+if\b`,
	`(?i)(//|#)\s*returns?(ing)?\s+the\b`,
	`(?i)(//|#)\s*creates?(ing)?\s+a\s+new\b`,
	`(?i)(//|#)\s*sets?(ting)?\s+the\b`,
	`(?i)(//|#)\s*(increment|decrement)(s|ing)?\s+the\b`,
	`(?i)(//|#)\s*calls?(ing)?\s+the\b`,
	`(?i)(//|#)\s*gets?(ting)?\s+the\b`,
	`(?i)(//|#)\s*(this function|this method|this class)\s+(does|is|handles|will)\b`,
}

// AICommentValidator flags filler comments that only restate adjacent code,
// a pattern common in LLM-generated output.
type AICommentValidator struct {
	validator.BaseValidator
	config   *config.AICommentValidatorConfig
	patterns []*regexp.Regexp
}

// NewAICommentValidator creates a new AICommentValidator.
func NewAICommentValidator(
	log logger.Logger,
	cfg *config.AICommentValidatorConfig,
	ruleAdapter validator.RuleChecker,
) *AICommentValidator {
	v := &AICommentValidator{
		BaseValidator: *validator.NewBaseValidatorWithRules("validate-ai-comments", log, ruleAdapter),
		config:        cfg,
	}

	v.patterns = compilePatterns(log, "AI comment", v.getPatterns())

	return v
}

// aiCommentHeader is the message header shown when a filler comment is blocked.
const aiCommentHeader = "Filler comments that only restate the code are not allowed"

// Validate checks for filler comments that restate adjacent code.
func (v *AICommentValidator) Validate(
	ctx context.Context,
	hookCtx *hook.Context,
) *validator.Result {
	return validatePatterns(
		ctx, hookCtx, v.CheckRules, v.patterns,
		aiCommentHeader, validator.RefAIComments,
	)
}

// getPatterns returns the configured patterns or defaults.
func (v *AICommentValidator) getPatterns() []string {
	if v.config != nil && len(v.config.Patterns) > 0 {
		return v.config.Patterns
	}

	return defaultAICommentPatterns
}

// Category returns the validator category for parallel execution.
// AICommentValidator uses CategoryCPU because it only does pattern matching.
func (*AICommentValidator) Category() validator.ValidatorCategory {
	return validator.CategoryCPU
}

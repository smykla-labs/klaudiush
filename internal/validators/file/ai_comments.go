package file

import (
	"context"
	"regexp"
	"strings"

	"github.com/smykla-skalski/klaudiush/internal/validator"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
)

// defaultAICommentPatterns flag a comment that opens with an action verb that
// merely restates the adjacent code — the dominant tell of LLM-generated
// filler. Idiomatic doc comments open with the identifier name and useful
// comments open with a reason ("why"), so neither trips these patterns; only
// the "verb-first narration of what the next line does" style does.
var defaultAICommentPatterns = []string{
	// A comment opening (optionally after "the"/"a") with a verb whose only
	// job is to describe the mechanics of the following statement.
	`(?i)(^\s*|\s)(//|#)\s*(the\s+|a\s+|an\s+)?` +
		`(initiali[sz]|set|reset|get|loop|iterate|check|return|` +
		`creat|mak|build|construct|instantiat|` +
		`call|invok|execut|run|increment|decrement|` +
		`add|append|prepend|insert|push|` +
		`remov|delet|clear|pop|drop|updat|modif|chang|` +
		`handl|process|pars|format|convert|encod|decod|` +
		`serializ|deserializ|marshal|unmarshal|comput|calculat|` +
		`assign|declar|defin|configur|register|setup|` +
		`open|clos|read|writ|load|sav|stor|fetch|send|receiv|` +
		`wait|start|stop|begin|print|log|output|emit|dispatch|` +
		`render|draw|filter|sort|find|search|count|` +
		`validat|verif|ensur|sanitiz|normaliz|` +
		`copy|mov|renam|split|join|trim|replac|compar|` +
		`toggl|enabl|disabl|mark|lock|bind|connect)` +
		`(e|es|ed|d|s|ing|ping|ting|ning|ling|ging|ies|ied|y)?\b`,
	// "This function/method/... does/is/handles/..." restatements.
	`(?i)(^\s*|\s)(//|#)\s*this\s+(function|method|class|struct|interface|type|` +
		`variable|field|constant|value|helper|wrapper)\s+` +
		`(does|is|are|will|handles?|returns?|sets?|gets?|` +
		`represents?|holds?|stores?|contains?)\b`,
}

// aiTodoMarker matches a comment that opens with a task or annotation marker.
// These carry intent ("what still needs doing") rather than restating code and
// are always allowed.
var aiTodoMarker = regexp.MustCompile(
	`(?i)^\s*(TODO|FIXME|HACK|XXX|BUG|WARNING|` +
		`OPTIMI[SZ]E|REVIEW|DEPRECATED)\b|^\s*@\w+`,
)

// aiExportedDecl matches a source line that declares an exported/public symbol.
// A leading comment block directly above such a line is its documentation and
// is allowed even when it opens with a verb (Go requires doc comments on
// exported identifiers).
var aiExportedDecl = regexp.MustCompile(
	`^\s*(` +
		`func\s+(\([^)]*\)\s*)?[A-Z]` + // Go exported func or method
		`|type\s+[A-Z]` + // Go exported type
		`|(const|var)\s+[A-Z]` + // Go exported const/var
		`|export\b` + // JS/TS export
		`|(async\s+)?(def|class)\s+[A-Za-z]` + // Python def/class (public: no leading _)
		`)`,
)

// aiCommentMarker locates the start of a // or # comment on a line. It anchors
// to line start or whitespace so markers inside string/URL literals (e.g. the
// "//" in "https://…") are not treated as comments.
var aiCommentMarker = regexp.MustCompile(`(^\s*|\s)(//|#)`)

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

// Validate checks for filler comments that restate adjacent code, allowing
// task markers and doc comments on exported declarations.
func (v *AICommentValidator) Validate(
	ctx context.Context,
	hookCtx *hook.Context,
) *validator.Result {
	if result := v.CheckRules(ctx, hookCtx); result != nil {
		return result
	}

	content := getWriteOrEditContent(hookCtx)
	if content == "" {
		return validator.Pass()
	}

	violations := findAICommentViolations(content, v.patterns)
	if len(violations) == 0 {
		return validator.Pass()
	}

	return validator.FailWithRef(
		validator.RefAIComments,
		formatPatternViolations(aiCommentHeader, violations),
	)
}

// findAICommentViolations reports filler comments while exempting task markers
// and documentation on exported declarations.
func findAICommentViolations(content string, patterns []*regexp.Regexp) []violation {
	var violations []violation

	lines := strings.Split(content, "\n")

	for i, line := range lines {
		loc := aiCommentMarker.FindStringIndex(line)
		if loc == nil {
			continue
		}

		if aiTodoMarker.MatchString(line[loc[1]:]) {
			continue
		}

		if isFullLineComment(line) && precedesExportedDecl(lines, i) {
			continue
		}

		for _, pattern := range patterns {
			if match := pattern.FindString(line); match != "" {
				violations = append(violations, violation{
					line:      i + 1,
					directive: strings.TrimSpace(match),
				})

				break
			}
		}
	}

	return violations
}

// isFullLineComment reports whether the line is a standalone comment rather
// than a trailing (inline) comment; only standalone comments can document a
// declaration.
func isFullLineComment(line string) bool {
	trimmed := strings.TrimSpace(line)

	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#")
}

// precedesExportedDecl reports whether the comment at index i is part of a
// leading comment block whose first non-comment line declares an exported
// symbol. A blank line breaks the association (it is no longer a doc comment).
func precedesExportedDecl(lines []string, i int) bool {
	for j := i + 1; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" {
			return false
		}

		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}

		return aiExportedDecl.MatchString(lines[j])
	}

	return false
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

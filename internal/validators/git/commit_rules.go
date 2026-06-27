package git

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/smykla-skalski/klaudiush/internal/validator"
)

// RuleResult contains the result of a rule validation including reference.
type RuleResult struct {
	// Message is the primary error (no emoji, no indentation).
	Message string

	// Context contains supplementary lines (no emoji, no indentation).
	Context []string

	// Reference is the URL that uniquely identifies this type of validation failure.
	Reference validator.Reference
}

// CommitRule represents a validation rule for commit messages.
type CommitRule interface {
	// Name returns the rule name.
	Name() string

	// Validate checks the commit against the rule and returns a RuleResult.
	Validate(commit *ParsedCommit, message string) *RuleResult
}

// TitleLengthRule validates the commit title length.
type TitleLengthRule struct {
	MaxLength                 int
	AllowUnlimitedRevertTitle bool
}

func (*TitleLengthRule) Name() string {
	return "title-length"
}

func (r *TitleLengthRule) Validate(commit *ParsedCommit, _ string) *RuleResult {
	// Skip length validation for revert commits if configured
	if r.AllowUnlimitedRevertTitle && isRevertCommit(commit.Title) {
		return nil
	}

	// Use rune count to properly handle Unicode characters
	titleLength := len([]rune(commit.Title))
	if titleLength <= r.MaxLength {
		return nil
	}

	return &RuleResult{
		Reference: validator.RefGitBadTitle,
		Message: fmt.Sprintf(
			"Title exceeds %d characters (%d chars): '%s'",
			r.MaxLength,
			titleLength,
			commit.Title,
		),
		Context: []string{
			"type(scope): prefix counts toward the limit",
			"Revert commits are exempt",
		},
	}
}

// ConventionalFormatRule validates conventional commit format.
type ConventionalFormatRule struct {
	ValidTypes   []string
	RequireScope bool
}

func (*ConventionalFormatRule) Name() string {
	return "conventional-format"
}

func (r *ConventionalFormatRule) Validate(commit *ParsedCommit, _ string) *RuleResult {
	// Skip validation for revert commits
	if isRevertCommit(commit.Title) {
		return nil
	}

	if !commit.Valid || commit.ParseError != "" {
		ctx := []string{}

		if r.RequireScope {
			ctx = append(ctx, "Scope is mandatory")
		}

		ctx = append(ctx,
			"Valid types: "+strings.Join(r.ValidTypes, ", "),
			"Alternative: Revert \"original commit title\"",
			fmt.Sprintf("Current title: '%s'", commit.Title),
			"type(scope): prefix counts toward 50-char limit",
		)

		return &RuleResult{
			Reference: validator.RefGitConventionalCommit,
			Message:   "Title doesn't follow conventional commits format: type(scope): description",
			Context:   ctx,
		}
	}

	// Check scope requirement
	if r.RequireScope && commit.Scope == "" {
		return &RuleResult{
			Reference: validator.RefGitConventionalCommit,
			Message:   "Title doesn't follow conventional commits format: type(scope): description",
			Context: []string{
				"Scope is mandatory",
				"Valid types: " + strings.Join(r.ValidTypes, ", "),
				"Alternative: Revert \"original commit title\"",
				fmt.Sprintf("Current title: '%s'", commit.Title),
				"type(scope): prefix counts toward 50-char limit",
			},
		}
	}

	return nil
}

// scopeOnlyTitleRegex matches "scope: description" format used by projects like home-manager.
// The scope can be any lowercase identifier with optional path separators (/, -, .).
var scopeOnlyTitleRegex = regexp.MustCompile(`^[a-z][a-z0-9./_-]*: .+`)

// ScopeOnlyFormatRule validates "scope: description" commit titles (no type prefix).
// This matches the convention used by home-manager, linux kernel patches, and similar
// projects where the scope is a module or file path, not a semantic type like feat/fix.
type ScopeOnlyFormatRule struct{}

func (*ScopeOnlyFormatRule) Name() string {
	return "scope-only-format"
}

func (*ScopeOnlyFormatRule) Validate(commit *ParsedCommit, _ string) *RuleResult {
	if isRevertCommit(commit.Title) {
		return nil
	}

	if scopeOnlyTitleRegex.MatchString(commit.Title) {
		return nil
	}

	return &RuleResult{
		Reference: validator.RefGitConventionalCommit,
		Message:   "Title doesn't follow scope-only format: scope: description",
		Context: []string{
			"Scope must start with a lowercase letter (a-z)",
			"Valid characters in scope: letters, digits, '.', '/', '_', '-'",
			"Examples: 'home-environment: use nix profile', 'modules/systemd: add unit'",
			fmt.Sprintf("Current title: '%s'", commit.Title),
		},
	}
}

// CustomPatternRule validates commit titles against a user-supplied regex.
type CustomPatternRule struct {
	Pattern *regexp.Regexp
}

// NewCustomPatternRule creates a CustomPatternRule from a regex string.
// Panics if the pattern is invalid (callers should validate first).
func NewCustomPatternRule(pattern string) *CustomPatternRule {
	return &CustomPatternRule{Pattern: regexp.MustCompile(pattern)}
}

func (*CustomPatternRule) Name() string {
	return "custom-pattern"
}

func (r *CustomPatternRule) Validate(commit *ParsedCommit, _ string) *RuleResult {
	if isRevertCommit(commit.Title) {
		return nil
	}

	if r.Pattern.MatchString(commit.Title) {
		return nil
	}

	return &RuleResult{
		Reference: validator.RefGitConventionalCommit,
		Message:   "Title doesn't match the required pattern",
		Context: []string{
			"Pattern: " + r.Pattern.String(),
			fmt.Sprintf("Current title: '%s'", commit.Title),
		},
	}
}

// InfraScopeMisuseRule blocks feat/fix with infrastructure scopes.
type InfraScopeMisuseRule struct {
	infraScopeMisuseRegex *regexp.Regexp
}

func NewInfraScopeMisuseRule() *InfraScopeMisuseRule {
	return &InfraScopeMisuseRule{
		infraScopeMisuseRegex: regexp.MustCompile(`^(feat|fix)\((ci|test|docs|build)\):`),
	}
}

func (*InfraScopeMisuseRule) Name() string {
	return "infra-scope-misuse"
}

func (r *InfraScopeMisuseRule) Validate(commit *ParsedCommit, _ string) *RuleResult {
	if !r.infraScopeMisuseRegex.MatchString(commit.Title) {
		return nil
	}

	matches := r.infraScopeMisuseRegex.FindStringSubmatch(commit.Title)

	const minMatchGroups = 3 // Full match + type + scope groups

	if len(matches) < minMatchGroups {
		return nil
	}

	typeMatch := matches[1]  // feat or fix
	scopeMatch := matches[2] // ci, test, docs, or build

	return &RuleResult{
		Reference: validator.RefGitFeatCI,
		Message: fmt.Sprintf(
			"Use '%s(...)' not '%s(%s)' for infrastructure changes",
			scopeMatch,
			typeMatch,
			scopeMatch,
		),
		Context: []string{
			"feat/fix should only be used for user-facing changes",
		},
	}
}

// BodyLineLengthRule validates body line lengths.
type BodyLineLengthRule struct {
	MaxLength int
	Tolerance int
	urlRegex  *regexp.Regexp
}

func NewBodyLineLengthRule(maxLength, tolerance int) *BodyLineLengthRule {
	return &BodyLineLengthRule{
		MaxLength: maxLength,
		Tolerance: tolerance,
		urlRegex:  regexp.MustCompile(`https?://`),
	}
}

func (*BodyLineLengthRule) Name() string {
	return "body-line-length"
}

func (r *BodyLineLengthRule) Validate(_ *ParsedCommit, commitMsg string) *RuleResult {
	lines := strings.Split(commitMsg, "\n")
	maxLenWithTolerance := r.MaxLength + r.Tolerance

	var primary string

	ctx := make([]string, 0)

	for lineNum, line := range lines {
		// Skip title (first line)
		if lineNum == 0 {
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Allow URLs to break the rule
		if r.urlRegex.MatchString(line) {
			continue
		}

		lineLen := len(line)
		if lineLen > maxLenWithTolerance {
			truncated := truncateLine(line)
			msg := fmt.Sprintf(
				"Line %d exceeds %d characters (%d chars, >%d over limit)",
				lineNum+1,
				r.MaxLength,
				lineLen,
				r.Tolerance,
			)

			if primary == "" {
				primary = msg

				ctx = append(ctx, fmt.Sprintf("Line: '%s'", truncated))
			} else {
				ctx = append(ctx, msg, fmt.Sprintf("Line: '%s'", truncated))
			}
		}
	}

	if primary == "" {
		return nil
	}

	return &RuleResult{
		Reference: validator.RefGitBadBody,
		Message:   primary,
		Context:   ctx,
	}
}

// ListFormattingRule validates list item formatting.
type ListFormattingRule struct {
	listItemRegex *regexp.Regexp
	trailerRegex  *regexp.Regexp
}

func NewListFormattingRule() *ListFormattingRule {
	return &ListFormattingRule{
		listItemRegex: regexp.MustCompile(`^\s*[-*]\s+|^\s*[0-9]+\.\s+`),
		trailerRegex:  regexp.MustCompile(`^[A-Za-z][-A-Za-z0-9 ]*:\s`),
	}
}

func (*ListFormattingRule) Name() string {
	return "list-formatting"
}

func (r *ListFormattingRule) Validate(_ *ParsedCommit, message string) *RuleResult {
	lines := strings.Split(message, "\n")
	prevLineEmpty := false
	foundFirstList := false

	for lineNum, line := range lines {
		// Skip title (first line)
		if lineNum == 0 {
			continue
		}

		// Check if blank line
		if strings.TrimSpace(line) == "" {
			prevLineEmpty = true

			continue
		}

		// Skip git trailer lines (Signed-off-by:, Co-authored-by:, etc.)
		if r.trailerRegex.MatchString(line) {
			continue
		}

		// Check for list items
		if r.listItemRegex.MatchString(line) {
			// Check if this is the first list item and there was no empty line before it
			if !foundFirstList && !prevLineEmpty {
				truncated := truncateLine(line)

				return &RuleResult{
					Reference: validator.RefGitListFormat,
					Message: fmt.Sprintf(
						"Missing empty line before first list item at line %d",
						lineNum+1,
					),
					Context: []string{
						"List items must be preceded by an empty line",
						fmt.Sprintf("Line: '%s'", truncated),
					},
				}
			}

			foundFirstList = true
		}

		prevLineEmpty = false
	}

	return nil
}

// PRReferenceRule blocks PR references in commit messages.
type PRReferenceRule struct {
	prReferenceRegex *regexp.Regexp
	hashRefRegex     *regexp.Regexp
	urlRefRegex      *regexp.Regexp
}

func NewPRReferenceRule() *PRReferenceRule {
	return &PRReferenceRule{
		prReferenceRegex: regexp.MustCompile(
			`#[0-9]{1,10}\b|(?:^|://|[^/a-zA-Z0-9])github\.com/[^/]+/[^/]+/pull/[0-9]{1,10}\b`,
		),
		hashRefRegex: regexp.MustCompile(`#[0-9]{1,10}\b`),
		urlRefRegex: regexp.MustCompile(
			`(?:^|://|[^/a-zA-Z0-9])github\.com/[^/]+/[^/]+/pull/[0-9]{1,10}\b`,
		),
	}
}

func (*PRReferenceRule) Name() string {
	return "pr-reference"
}

func (r *PRReferenceRule) Validate(_ *ParsedCommit, message string) *RuleResult {
	if !r.prReferenceRegex.MatchString(message) {
		return nil
	}

	ctx := make([]string, 0)

	// Show examples for hash references
	if hashMatch := r.hashRefRegex.FindString(message); hashMatch != "" {
		fix := strings.TrimPrefix(hashMatch, "#")
		ctx = append(ctx, fmt.Sprintf("Found: '%s' -> Should be: '%s'", hashMatch, fix))
	}

	// Show examples for URL references
	if urlMatch := r.urlRefRegex.FindString(message); urlMatch != "" {
		prNumRegex := regexp.MustCompile(`[0-9]{1,10}$`)
		prNum := prNumRegex.FindString(urlMatch)

		// Strip any prefix captured by the anchor pattern (e.g., "://", space, etc.)
		cleanURL := urlMatch
		if idx := strings.Index(urlMatch, "github.com"); idx > 0 {
			cleanURL = urlMatch[idx:]
		}

		ctx = append(
			ctx,
			fmt.Sprintf("Found: 'https://%s' -> Should be: '%s'", cleanURL, prNum),
		)
	}

	return &RuleResult{
		Reference: validator.RefGitPRRef,
		Message:   "PR references found - remove '#' prefix or convert URLs to plain numbers",
		Context:   ctx,
	}
}

// AIAttributionRule blocks AI attribution patterns.
type AIAttributionRule struct{}

func NewAIAttributionRule() *AIAttributionRule {
	return &AIAttributionRule{}
}

func (*AIAttributionRule) Name() string {
	return "ai-attribution"
}

func (*AIAttributionRule) Validate(_ *ParsedCommit, message string) *RuleResult {
	if !containsAIAttribution(message) {
		return nil
	}

	return &RuleResult{
		Reference: validator.RefGitClaudeAttr,
		Message:   "Commit message contains AI attribution - remove any AI generation attribution",
	}
}

// ForbiddenPatternRule blocks forbidden patterns in commit messages.
type ForbiddenPatternRule struct {
	Patterns []string
}

func (*ForbiddenPatternRule) Name() string {
	return "forbidden-pattern"
}

func (r *ForbiddenPatternRule) Validate(_ *ParsedCommit, message string) *RuleResult {
	if len(r.Patterns) == 0 {
		return nil
	}

	var primary string

	ctx := make([]string, 0)

	for _, pattern := range r.Patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		if re.MatchString(message) {
			match := re.FindString(message)
			msg := fmt.Sprintf("Forbidden pattern found: '%s'", match)

			if primary == "" {
				primary = msg

				ctx = append(ctx, "Pattern: "+pattern)
			} else {
				ctx = append(ctx, msg, "Pattern: "+pattern)
			}
		}
	}

	if primary == "" {
		return nil
	}

	return &RuleResult{
		Reference: validator.RefGitForbiddenPattern,
		Message:   primary,
		Context:   ctx,
	}
}

// SignoffRule validates the Signed-off-by trailer.
type SignoffRule struct {
	ExpectedSignoff string
}

func (*SignoffRule) Name() string {
	return "signoff"
}

func (r *SignoffRule) Validate(_ *ParsedCommit, message string) *RuleResult {
	if r.ExpectedSignoff == "" {
		return nil
	}

	if !strings.Contains(message, "Signed-off-by:") {
		return nil
	}

	lines := strings.Split(message, "\n")
	signoffLine := ""

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Signed-off-by:") {
			signoffLine = strings.TrimSpace(line)

			break
		}
	}

	expectedSignoffLine := "Signed-off-by: " + r.ExpectedSignoff
	if signoffLine != expectedSignoffLine {
		return &RuleResult{
			Reference: validator.RefGitSignoffMismatch,
			Message:   "Wrong signoff identity",
			Context: []string{
				"Found: " + signoffLine,
				"Expected: " + expectedSignoffLine,
			},
		}
	}

	return nil
}

// aiAttributionLinkPattern matches a markdown link whose label and target both
// mention a known AI assistant, e.g.
// "[Claude Code](https://claude.com/claude-code)",
// "[GitHub Copilot](https://github.com/features/copilot)", or
// "[Codex](https://openai.com/codex)". The label is matched anywhere so a
// backtick- or text-prefixed label cannot slip the check.
var aiAttributionLinkPattern = regexp.MustCompile(
	`\[[^\]]*(claude|copilot|codex)[^\]]*\]\([^)]*(claude|copilot|codex)[^)]*\)`,
)

var aiAssistantNames = []string{
	"claude",
	"claude code",
	"github copilot",
	"copilot",
	"openai codex",
	"codex",
}

var aiAttributionPhrases = []string{
	"generated by ",
	"generated with ",
	"created by ",
	"written by ",
	"authored by ",
	"assisted by ",
	"with help from ",
	"powered by ",
}

// containsAIAttribution reports whether a message carries AI generation
// attribution. It matches explicit attribution phrases and markdown footer
// links, while leaving allow-listed references untouched - the CLAUDE.md
// guidance file, the klaudiush tool, and claude-hooks - including in
// markdown-link form such as "[CLAUDE.md](CLAUDE.md)".
func containsAIAttribution(message string) bool {
	lower := strings.ReplaceAll(strings.ToLower(message), `\n`, "\n")

	if containsExplicitAIAttribution(lower) {
		return true
	}

	if containsAIAttributionLink(lower) {
		return true
	}

	return false
}

func containsExplicitAIAttribution(lower string) bool {
	for _, phrase := range aiAttributionPhrases {
		for _, assistant := range aiAssistantNames {
			if strings.Contains(lower, phrase+assistant) {
				return true
			}
		}
	}

	for _, assistant := range []string{"claude", "copilot", "codex"} {
		if strings.Contains(lower, assistant+" ai") {
			return true
		}
	}

	for line := range strings.SplitSeq(lower, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "co-authored-by:") &&
			containsAIAssistantName(line) {
			return true
		}
	}

	return false
}

func containsAIAssistantName(lower string) bool {
	for _, assistant := range aiAssistantNames {
		if strings.Contains(lower, assistant) {
			return true
		}
	}

	return false
}

func containsAIAttributionLink(lower string) bool {
	for line := range strings.SplitSeq(lower, "\n") {
		if !containsAttributionPhrase(line) {
			continue
		}

		// The allow-list is applied per matched link rather than to the whole
		// message, so a real footer cannot slip through just because the body
		// also mentions klaudiush or CLAUDE.md elsewhere.
		for _, link := range aiAttributionLinkPattern.FindAllString(line, -1) {
			if !isLegitimateAIReference(link) {
				return true
			}
		}
	}

	return false
}

func containsAttributionPhrase(lower string) bool {
	for _, phrase := range aiAttributionPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}

	return false
}

// isLegitimateAIReference reports whether a lowercased markdown link that
// mentions an AI assistant points at a known non-attribution reference - the
// CLAUDE.md guidance file, the klaudiush tool, or claude-hooks - rather than
// attribution.
// Only concrete file/tool identifiers are allow-listed: inline-code heuristics
// are deliberately excluded, since a footer label wrapped in backticks could
// otherwise slip the attribution check.
func isLegitimateAIReference(lower string) bool {
	legitimatePatterns := []string{
		"claude.md",
		"claude-hooks",
		"klaudiush",
	}

	for _, pattern := range legitimatePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

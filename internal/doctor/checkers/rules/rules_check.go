// Package ruleschecker provides checkers for validation rules configuration.
package ruleschecker

import (
	"context"
	"fmt"
	"slices"
	"strings"

	internalconfig "github.com/smykla-labs/klaudiush/internal/config"
	"github.com/smykla-labs/klaudiush/internal/doctor"
	"github.com/smykla-labs/klaudiush/pkg/config"
)

// Valid values for rules configuration.
var (
	validActionTypes = []string{"allow", "block", "warn"}
	validEventTypes  = []string{"PreToolUse", "PostToolUse", "Notification"}
	validToolTypes   = []string{"Bash", "Write", "Edit", "MultiEdit", "Grep", "Read", "Glob"}
)

// RuleIssue represents an issue found in a rule configuration.
type RuleIssue struct {
	RuleIndex int
	RuleName  string
	IssueType string
	Message   string
	Fixable   bool
}

// ConfigLoader defines the interface for configuration loading operations.
type ConfigLoader interface {
	HasProjectConfig() bool
	Load(flags map[string]any) (*config.Config, error)
	LoadWithoutValidation(flags map[string]any) (*config.Config, error)
}

// RulesChecker checks the validity of rules configuration.
type RulesChecker struct {
	loader ConfigLoader
	issues []RuleIssue
}

// NewRulesChecker creates a new rules checker.
func NewRulesChecker() *RulesChecker {
	loader, _ := internalconfig.NewKoanfLoader()

	return &RulesChecker{
		loader: loader,
	}
}

// NewRulesCheckerWithLoader creates a RulesChecker with a custom loader (for testing).
func NewRulesCheckerWithLoader(loader ConfigLoader) *RulesChecker {
	return &RulesChecker{
		loader: loader,
	}
}

// Name returns the name of the check.
func (*RulesChecker) Name() string {
	return "Rules validation"
}

// Category returns the category of the check.
func (*RulesChecker) Category() doctor.Category {
	return doctor.CategoryConfig
}

// GetIssues returns the issues found during the last check.
func (c *RulesChecker) GetIssues() []RuleIssue {
	return c.issues
}

// Check performs the rules validation check.
func (c *RulesChecker) Check(_ context.Context) doctor.CheckResult {
	c.issues = nil

	if !c.loader.HasProjectConfig() {
		return doctor.Skip("Rules validation", "No project config found")
	}

	// Use LoadWithoutValidation to allow checking invalid rules
	cfg, err := c.loader.LoadWithoutValidation(nil)
	if err != nil {
		// Config loading errors are handled by config checker
		return doctor.Skip("Rules validation", "Config load failed (see config check)")
	}

	if cfg.Rules == nil || len(cfg.Rules.Rules) == 0 {
		return doctor.Pass("Rules validation", "No rules configured")
	}

	// Validate each enabled rule
	enabledCount := 0

	for i := range cfg.Rules.Rules {
		// Skip validation for disabled rules
		if !cfg.Rules.Rules[i].IsRuleEnabled() {
			continue
		}

		enabledCount++

		c.validateRule(i, &cfg.Rules.Rules[i])
	}

	if len(c.issues) == 0 {
		msg := fmt.Sprintf("%d rule(s) validated", enabledCount)

		return doctor.Pass("Rules validation", msg)
	}

	// Build details from issues
	details := make([]string, 0, len(c.issues)+1)

	for _, issue := range c.issues {
		var prefix string
		if issue.RuleName != "" {
			prefix = fmt.Sprintf("Rule %q", issue.RuleName)
		} else {
			prefix = fmt.Sprintf("Rule #%d", issue.RuleIndex+1)
		}

		details = append(details, fmt.Sprintf("%s: %s", prefix, issue.Message))
	}

	// Count fixable issues
	fixableCount := 0

	for _, issue := range c.issues {
		if issue.Fixable {
			fixableCount++
		}
	}

	result := doctor.FailError("Rules validation",
		fmt.Sprintf("%d invalid rule(s) found", len(c.issues))).
		WithDetails(details...)

	if fixableCount > 0 {
		result = result.WithFixID("fix_invalid_rules")
	}

	return result
}

// validateRule validates a single rule and records issues.
func (c *RulesChecker) validateRule(index int, rule *config.RuleConfig) {
	ruleName := rule.Name

	// Check for missing match section
	if rule.Match == nil {
		c.issues = append(c.issues, RuleIssue{
			RuleIndex: index,
			RuleName:  ruleName,
			IssueType: "no_match_section",
			Message:   "missing match section (rule will never match)",
			Fixable:   true,
		})

		return // No point checking other fields if match is missing
	}

	// Check for empty match conditions
	if !hasMatchConditions(rule.Match) {
		c.issues = append(c.issues, RuleIssue{
			RuleIndex: index,
			RuleName:  ruleName,
			IssueType: "empty_match",
			Message:   "match section is empty (rule will never match)",
			Fixable:   true,
		})
	}

	// Check for invalid event_type
	if rule.Match.EventType != "" {
		if !containsCaseInsensitive(validEventTypes, rule.Match.EventType) {
			c.issues = append(c.issues, RuleIssue{
				RuleIndex: index,
				RuleName:  ruleName,
				IssueType: "invalid_event_type",
				Message: fmt.Sprintf("invalid event_type %q (valid: %s)",
					rule.Match.EventType, strings.Join(validEventTypes, ", ")),
				Fixable: true,
			})
		}
	}

	// Check for invalid tool_type
	if rule.Match.ToolType != "" {
		if !containsCaseInsensitive(validToolTypes, rule.Match.ToolType) {
			c.issues = append(c.issues, RuleIssue{
				RuleIndex: index,
				RuleName:  ruleName,
				IssueType: "invalid_tool_type",
				Message: fmt.Sprintf("invalid tool_type %q (valid: %s)",
					rule.Match.ToolType, strings.Join(validToolTypes, ", ")),
				Fixable: true,
			})
		}
	}

	// Check for invalid action type
	if rule.Action != nil && rule.Action.Type != "" {
		if !slices.Contains(validActionTypes, rule.Action.Type) {
			c.issues = append(c.issues, RuleIssue{
				RuleIndex: index,
				RuleName:  ruleName,
				IssueType: "invalid_action_type",
				Message: fmt.Sprintf("invalid action type %q (valid: %s)",
					rule.Action.Type, strings.Join(validActionTypes, ", ")),
				Fixable: true,
			})
		}
	}
}

// hasMatchConditions checks if a rule has at least one match condition.
func hasMatchConditions(match *config.RuleMatchConfig) bool {
	if match == nil {
		return false
	}

	return match.ValidatorType != "" ||
		match.RepoPattern != "" ||
		len(match.RepoPatterns) > 0 ||
		match.Remote != "" ||
		match.BranchPattern != "" ||
		len(match.BranchPatterns) > 0 ||
		match.FilePattern != "" ||
		len(match.FilePatterns) > 0 ||
		match.ContentPattern != "" ||
		len(match.ContentPatterns) > 0 ||
		match.CommandPattern != "" ||
		len(match.CommandPatterns) > 0 ||
		match.ToolType != "" ||
		match.EventType != ""
}

// containsCaseInsensitive checks if a string exists in a slice (case-insensitive).
func containsCaseInsensitive(slice []string, target string) bool {
	targetLower := strings.ToLower(target)

	for _, s := range slice {
		if strings.ToLower(s) == targetLower {
			return true
		}
	}

	return false
}

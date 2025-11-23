package git

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	validTypesPattern         = "build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test"
	nonUserFacingTypesPattern = "ci|test|chore|build|docs|style|refactor"
)

var (
	semanticCommitRegex = regexp.MustCompile(
		fmt.Sprintf(`^(%s)(\([a-zA-Z0-9_\/-]+\))?!?: .+`, validTypesPattern),
	)
	userFacingInfraRegex = regexp.MustCompile(`^(feat|fix)\((ci|test|docs|build)\):`)
)

// PRTitleValidationResult contains the result of PR title validation
type PRTitleValidationResult struct {
	Valid        bool
	ErrorMessage string
	Details      []string
}

// ValidatePRTitle validates that a PR title follows semantic commit format
// and doesn't misuse feat/fix with infrastructure scopes
func ValidatePRTitle(title string) PRTitleValidationResult {
	if title == "" {
		return PRTitleValidationResult{
			Valid:        false,
			ErrorMessage: "PR title is empty",
		}
	}

	// Check semantic commit format
	if !semanticCommitRegex.MatchString(title) {
		return PRTitleValidationResult{
			Valid:        false,
			ErrorMessage: "PR title doesn't follow semantic commit format",
			Details: []string{
				fmt.Sprintf("Current: '%s'", title),
				"Expected: type(scope): description",
				"Valid types: build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test",
			},
		}
	}

	// Check for feat/fix misuse with infrastructure scopes
	if matches := userFacingInfraRegex.FindStringSubmatch(title); matches != nil {
		typeMatch := matches[1]  // feat or fix
		scopeMatch := matches[2] // ci, test, docs, or build

		return PRTitleValidationResult{
			Valid:        false,
			ErrorMessage: fmt.Sprintf("Use '%s(...)' not '%s(%s)' for infrastructure changes", scopeMatch, typeMatch, scopeMatch),
			Details: []string{
				"feat/fix should only be used for user-facing changes",
			},
		}
	}

	return PRTitleValidationResult{Valid: true}
}

// ExtractPRType extracts the type from a semantic commit title (e.g., "feat", "fix", "ci")
func ExtractPRType(title string) string {
	typeRegex := regexp.MustCompile(fmt.Sprintf(`^(%s)`, validTypesPattern))

	matches := typeRegex.FindStringSubmatch(title)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// IsNonUserFacingType returns true if the type is non-user-facing
// (ci, test, chore, build, docs, style, refactor)
func IsNonUserFacingType(prType string) bool {
	nonUserFacingTypes := strings.Split(nonUserFacingTypesPattern, "|")
	return slices.Contains(nonUserFacingTypes, prType)
}

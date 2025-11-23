package git

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	motivationHeader       = "## Motivation"
	implementationHeader   = "## Implementation information"
	supportingDocsHeader   = "## Supporting documentation"
	changelogLineThreshold = 40
	shortLineThreshold     = 3
	totalLineThreshold     = 5
)

var (
	changelogSkipRegex   = regexp.MustCompile(`(?m)^>\s*Changelog:\s*skip`)
	changelogCustomRegex = regexp.MustCompile(`(?m)^>\s*Changelog:\s*(.+)`)
	formalWordsRegex     = regexp.MustCompile(`(?i)\b(utilize|leverage|facilitate|implement)\b`)
)

// PRBodyValidationResult contains the result of PR body validation
type PRBodyValidationResult struct {
	Errors   []string
	Warnings []string
}

// ValidatePRBody validates PR body structure, changelog rules, and language
func ValidatePRBody(body, prType string) PRBodyValidationResult {
	result := PRBodyValidationResult{
		Errors:   []string{},
		Warnings: []string{},
	}

	if body == "" {
		result.Warnings = append(result.Warnings, "Could not extract PR body - ensure you're using --body flag")
		return result
	}

	// Check for required sections
	checkRequiredSections(body, &result)

	// Validate changelog handling
	validateChangelog(body, prType, &result)

	// Check for simple, personal language
	if formalWordsRegex.MatchString(body) {
		result.Warnings = append(result.Warnings,
			"PR description uses formal language - consider simpler, more personal tone",
			"Examples: 'use' instead of 'utilize', 'add' instead of 'implement'",
		)
	}

	// Check for line breaks in paragraphs
	checkLineBreaks(body, &result)

	// Check if Supporting documentation section is empty
	checkSupportingDocs(body, &result)

	return result
}

// checkRequiredSections validates that all required sections are present
func checkRequiredSections(body string, result *PRBodyValidationResult) {
	if !strings.Contains(body, motivationHeader) {
		result.Errors = append(result.Errors, "PR body missing '## Motivation' section")
	}

	if !strings.Contains(body, implementationHeader) {
		result.Errors = append(result.Errors, "PR body missing '## Implementation information' section")
	}

	if !strings.Contains(body, supportingDocsHeader) {
		result.Errors = append(result.Errors, "PR body missing '## Supporting documentation' section")
	}
}

// validateChangelog validates changelog rules based on PR type
func validateChangelog(body, prType string, result *PRBodyValidationResult) {
	hasChangelogSkip := changelogSkipRegex.MatchString(body)
	changelogMatches := changelogCustomRegex.FindStringSubmatch(body)
	hasCustomChangelog := len(changelogMatches) > 1 && changelogMatches[1] != "skip"

	if prType != "" {
		isNonUserFacing := IsNonUserFacingType(prType)

		// Non-user-facing changes should have changelog: skip
		if isNonUserFacing && !hasChangelogSkip && !hasCustomChangelog {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("PR type '%s' should typically have '> Changelog: skip'", prType),
				"Infrastructure changes don't need changelog entries",
			)
		}

		// User-facing changes should NOT skip changelog
		if !isNonUserFacing && hasChangelogSkip {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("PR type '%s' is user-facing but has 'Changelog: skip'", prType),
				"Consider removing 'skip' or using custom changelog entry",
			)
		}
	}

	// Validate custom changelog format if present
	if hasCustomChangelog {
		changelogEntry := changelogMatches[1]
		if !semanticCommitRegex.MatchString(changelogEntry) {
			result.Errors = append(result.Errors,
				"Custom changelog entry doesn't follow semantic commit format",
				fmt.Sprintf("Found: '%s'", changelogEntry),
				"Note: Changelog format is flexible on length but should be semantic",
			)
		}
	}
}

// checkLineBreaks checks for unnecessary line breaks in paragraphs
func checkLineBreaks(body string, result *PRBodyValidationResult) {
	shortLines := 0
	totalLines := 0
	lines := strings.SplitSeq(body, "\n")

	for line := range lines {
		// Skip headers, blank lines, blockquotes, and list items
		if strings.HasPrefix(line, "##") ||
			strings.TrimSpace(line) == "" ||
			strings.HasPrefix(line, ">") ||
			strings.HasPrefix(line, "-") ||
			strings.HasPrefix(line, "*") {
			continue
		}

		totalLines++
		if len(line) < changelogLineThreshold {
			shortLines++
		}
	}

	if totalLines > totalLineThreshold && shortLines > shortLineThreshold {
		result.Warnings = append(result.Warnings,
			"PR description may have unnecessary line breaks within paragraphs",
			"Don't break long lines in body paragraphs - let them flow naturally",
		)
	}
}

// checkSupportingDocs checks if Supporting documentation section is empty or N/A
func checkSupportingDocs(body string, result *PRBodyValidationResult) {
	idx := strings.Index(body, supportingDocsHeader)
	if idx == -1 {
		return
	}

	afterHeader := body[idx+len(supportingDocsHeader):]
	lines := strings.Split(afterHeader, "\n")

	// Check first few non-empty lines after header
	isEmpty := true

	for i := 0; i < len(lines) && i < 5; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" &&
			!strings.HasPrefix(trimmed, "##") &&
			!strings.EqualFold(trimmed, "n/a") &&
			!strings.EqualFold(trimmed, "none") {
			isEmpty = false
			break
		}
	}

	if isEmpty {
		result.Warnings = append(result.Warnings,
			"Supporting documentation section is empty or N/A",
			"Consider removing the section entirely if there's no supporting documentation",
		)
	}
}

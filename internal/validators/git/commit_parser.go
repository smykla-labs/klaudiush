package git

import (
	"regexp"
	"strings"

	conventionalcommits "github.com/leodido/go-conventionalcommits"
	ccp "github.com/leodido/go-conventionalcommits/parser"
)

// footerPattern matches git trailer format: "Token: value"
// Compiled once at package initialization for efficiency.
var footerPattern = regexp.MustCompile(`^([A-Za-z0-9-]+):\s*(.*)$`)

// ParsedCommit represents a parsed conventional commit message.
type ParsedCommit struct {
	// Type is the commit type (e.g., "feat", "fix", "chore").
	Type string

	// Scope is the optional scope (e.g., "api", "auth").
	Scope string

	// Description is the commit description.
	Description string

	// Body is the optional commit body.
	Body string

	// Footers contains any footer tokens/values.
	Footers map[string][]string

	// IsBreakingChange indicates if this is a breaking change.
	IsBreakingChange bool

	// Title is the full first line (type(scope): description).
	Title string

	// Raw is the original commit message.
	Raw string

	// Valid indicates whether the commit follows conventional commit format.
	Valid bool

	// ParseError contains the error message if parsing failed.
	ParseError string
}

// CommitParser parses conventional commit messages.
type CommitParser struct {
	machine    conventionalcommits.Machine
	validTypes map[string]bool
}

// CommitParserOption configures the CommitParser.
type CommitParserOption func(*CommitParser)

// WithValidTypes sets the allowed commit types.
func WithValidTypes(types []string) CommitParserOption {
	return func(p *CommitParser) {
		p.validTypes = make(map[string]bool, len(types))
		for _, t := range types {
			p.validTypes[t] = true
		}
	}
}

// NewCommitParser creates a new CommitParser with the given options.
func NewCommitParser(opts ...CommitParserOption) *CommitParser {
	p := &CommitParser{
		machine: ccp.NewMachine(
			ccp.WithTypes(conventionalcommits.TypesFreeForm),
			ccp.WithBestEffort(),
		),
		validTypes: make(map[string]bool),
	}

	// Set default valid types
	for _, t := range defaultValidTypes {
		p.validTypes[t] = true
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Parse parses a commit message into a structured ParsedCommit.
func (p *CommitParser) Parse(message string) *ParsedCommit {
	result := &ParsedCommit{
		Raw: message,
	}

	if message == "" {
		return result
	}

	// Extract title (first line)
	title := extractTitle(message)
	result.Title = title

	// Check for git revert format first
	if isRevertCommit(title) {
		result.Valid = true
		result.Type = "revert"

		return result
	}

	// Try parsing the full message first
	msg, err := p.machine.Parse([]byte(message))
	usedFallback := false

	// Handle parse errors with fallback for trailer validation issues
	if err != nil {
		msg, usedFallback, err = p.handleParseError(err, title, message, result)
		if err != nil {
			result.ParseError = err.Error()
			return result
		}
	}

	// Type assertion to access the conventional commit
	cc, ok := msg.(*conventionalcommits.ConventionalCommit)
	if !ok || cc == nil {
		result.ParseError = "failed to parse as conventional commit"

		return result
	}

	// Extract parsed fields
	result.Type = cc.Type
	result.Description = cc.Description
	result.IsBreakingChange = cc.Exclamation

	if cc.Scope != nil {
		result.Scope = *cc.Scope
	}

	// Only extract body/footers from the library if we didn't use fallback mode.
	// In fallback mode, we already manually extracted these via extractBodyAndFooters().
	if !usedFallback {
		p.extractLibraryBodyAndFooters(cc, result)
	}

	// Validate type against allowed types
	if len(p.validTypes) > 0 && !p.validTypes[result.Type] {
		result.ParseError = "invalid commit type: " + result.Type
		result.Valid = false

		return result
	}

	result.Valid = true

	return result
}

// handleParseError handles parsing errors with fallback for trailer validation issues.
//
// NOTE: This depends on the error message format from go-conventionalcommits.
// The library returns errors like "illegal ',' character in trailer: col=533" when
// it encounters invalid trailer syntax. We detect this by checking for "trailer" in
// the error message. If the library changes its error messages, this may break.
//
//nolint:ireturn // Returns interface type from go-conventionalcommits library
func (p *CommitParser) handleParseError(
	err error,
	title string,
	message string,
	result *ParsedCommit,
) (conventionalcommits.Message, bool, error) {
	// Only use fallback for trailer validation errors
	if !strings.Contains(err.Error(), "trailer") {
		return nil, false, err
	}

	// Parse just the title line to get type, scope, description
	msg, titleErr := p.machine.Parse([]byte(title))
	if titleErr != nil {
		return nil, false, titleErr
	}

	// Manually extract body and footers since the library rejected the full message
	p.extractBodyAndFooters(message, result)

	return msg, true, nil
}

// extractLibraryBodyAndFooters extracts body and footers from the library's parsed result.
func (*CommitParser) extractLibraryBodyAndFooters(
	cc *conventionalcommits.ConventionalCommit,
	result *ParsedCommit,
) {
	if cc.Body != nil {
		result.Body = *cc.Body
	}

	if cc.Footers != nil {
		result.Footers = cc.Footers

		// Check for BREAKING CHANGE footer
		if _, hasBreaking := cc.Footers["BREAKING CHANGE"]; hasBreaking {
			result.IsBreakingChange = true
		}

		if _, hasBreaking := cc.Footers["BREAKING-CHANGE"]; hasBreaking {
			result.IsBreakingChange = true
		}
	}
}

// extractBodyAndFooters manually extracts body and footers from the full message
// when the library's parser fails due to strict trailer validation.
//
// This function maintains consistency with the full parser by populating the Footers
// map and detecting BREAKING CHANGE footers, ensuring the same behavior regardless
// of which parsing path was taken.
func (*CommitParser) extractBodyAndFooters(message string, result *ParsedCommit) {
	lines := strings.Split(message, "\n")
	if len(lines) <= 1 {
		return
	}

	// Skip title and any blank lines after it
	bodyStartIdx := 1
	for bodyStartIdx < len(lines) && strings.TrimSpace(lines[bodyStartIdx]) == "" {
		bodyStartIdx++
	}

	if bodyStartIdx >= len(lines) {
		return
	}

	bodyLines := lines[bodyStartIdx:]

	// Find where footers start (scanning backwards from the end)
	footerStartIdx := findFooterStartIndex(bodyLines)

	// Extract footers if found
	if footerStartIdx < len(bodyLines) {
		extractFootersFromLines(bodyLines[footerStartIdx:], result)
		result.Body = strings.TrimRight(strings.Join(bodyLines[:footerStartIdx], "\n"), "\n")
	} else {
		result.Body = strings.Join(bodyLines, "\n")
	}
}

// findFooterStartIndex scans backwards to find where git trailers start in the body.
//
// Git trailers typically appear at the end, separated from the body by a blank line.
// However, if all body lines match the footer pattern (no blank line separator), we
// treat the entire body as footers to handle edge cases where commits contain only
// trailers without body text.
func findFooterStartIndex(bodyLines []string) int {
	footerStartIdx := len(bodyLines)
	foundNonFooter := false

	for i := len(bodyLines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(bodyLines[i])
		if line == "" {
			// Blank line marks the boundary
			footerStartIdx = i + 1
			break
		}

		if !footerPattern.MatchString(line) {
			// Non-footer line found
			footerStartIdx = i + 1
			foundNonFooter = true

			break
		}
	}

	// If we scanned all lines and they all matched the footer pattern,
	// treat the entire body as footers (footerStartIdx remains 0 from the loop)
	if !foundNonFooter && footerStartIdx == len(bodyLines) {
		footerStartIdx = 0
	}

	return footerStartIdx
}

// extractFootersFromLines parses footer lines and populates the result's Footers map.
func extractFootersFromLines(footerLines []string, result *ParsedCommit) {
	if result.Footers == nil {
		result.Footers = make(map[string][]string)
	}

	const expectedFooterMatches = 3 // full match + 2 capture groups

	for _, line := range footerLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		matches := footerPattern.FindStringSubmatch(trimmedLine)
		if len(matches) != expectedFooterMatches {
			continue
		}

		token := matches[1]
		value := matches[2]
		result.Footers[token] = append(result.Footers[token], value)

		// Check for breaking change markers
		if token == "BREAKING CHANGE" || token == "BREAKING-CHANGE" {
			result.IsBreakingChange = true
		}
	}
}

// IsValidType checks if a type is in the valid types list.
func (p *CommitParser) IsValidType(commitType string) bool {
	if len(p.validTypes) == 0 {
		return true
	}

	return p.validTypes[commitType]
}

// GetValidTypes returns the list of valid types.
func (p *CommitParser) GetValidTypes() []string {
	types := make([]string, 0, len(p.validTypes))
	for t := range p.validTypes {
		types = append(types, t)
	}

	return types
}

// extractTitle extracts the first non-empty line from a message.
func extractTitle(message string) string {
	// Find the first newline or end of string
	for i, c := range message {
		if c == '\n' {
			return message[:i]
		}
	}

	return message
}

// conventionalTitleRegex matches conventional commit title format.
var conventionalTitleRegex = regexp.MustCompile(`^(\w+)(\([a-zA-Z0-9_\/-]+\))?!?: .+`)

// HasValidFormat checks if a title matches the conventional commit format.
func HasValidFormat(title string) bool {
	return conventionalTitleRegex.MatchString(title)
}

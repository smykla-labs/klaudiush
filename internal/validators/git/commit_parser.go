package git

import (
	"regexp"

	conventionalcommits "github.com/leodido/go-conventionalcommits"
	ccp "github.com/leodido/go-conventionalcommits/parser"
)

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

	// Parse with go-conventionalcommits
	msg, err := p.machine.Parse([]byte(message))
	if err != nil {
		result.ParseError = err.Error()

		return result
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

	// Validate type against allowed types
	if len(p.validTypes) > 0 && !p.validTypes[result.Type] {
		result.ParseError = "invalid commit type: " + result.Type
		result.Valid = false

		return result
	}

	result.Valid = true

	return result
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

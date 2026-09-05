// Package config provides configuration schema types for klaudiush validators.
package config

// GitHubConfig groups all GitHub CLI-related validator configurations.
type GitHubConfig struct {
	// Issue validator configuration
	Issue *IssueValidatorConfig `json:"issue,omitempty" koanf:"issue" toml:"issue,omitempty"`

	// API validator configuration
	API *APIValidatorConfig `json:"api,omitempty" koanf:"api" toml:"api,omitempty"`
}

// APIValidatorConfig configures the gh api validator, which rejects calls to
// GitHub endpoints that create commits outside git and therefore outside the
// commit validators.
type APIValidatorConfig struct {
	ValidatorConfig `koanf:",squash"`

	// BlockedEndpoints lists "METHOD path-pattern" entries. The method may be
	// "*" to match any. The pattern is matched against the normalized endpoint
	// path (no scheme, host, leading slash or query string) and is a glob
	// unless it contains regex syntax.
	// Default: the REST endpoints that create commits.
	BlockedEndpoints []string `json:"blocked_endpoints,omitempty" koanf:"blocked_endpoints" toml:"blocked_endpoints,omitempty"`

	// BlockedGraphQLMutations lists mutation names rejected when found in the
	// query body of a /graphql request.
	// Default: ["createCommitOnBranch"]
	BlockedGraphQLMutations []string `json:"blocked_graphql_mutations,omitempty" koanf:"blocked_graphql_mutations" toml:"blocked_graphql_mutations,omitempty"`
}

// IssueValidatorConfig configures the gh issue create validator.
type IssueValidatorConfig struct {
	ValidatorConfig `koanf:",squash"`

	// RequireBody requires issue body to be present.
	// Default: false (body is optional for issues)
	RequireBody *bool `json:"require_body,omitempty" koanf:"require_body" toml:"require_body,omitempty"`

	// MarkdownDisabledRules is a list of markdownlint rules to disable for issue body validation.
	// Default: ["MD013", "MD034", "MD041", "MD047"]
	// - MD013: Line length (issues often have long lines)
	// - MD034: Bare URLs (commonly used in issues)
	// - MD041: First line heading (issues often start with ### headings)
	// - MD047: Files should end with newline (gh CLI handles this)
	MarkdownDisabledRules []string `json:"markdown_disabled_rules,omitempty" koanf:"markdown_disabled_rules" toml:"markdown_disabled_rules,omitempty"`

	// Timeout for markdown linting operations.
	// Default: 10s
	Timeout Duration `json:"timeout,omitempty" koanf:"timeout" toml:"timeout,omitempty"`
}

// DefaultBlockedGHAPIEndpoints returns the REST endpoints that create a commit
// without running git: writing or deleting repository contents, creating a git
// commit object, merging a branch, and merging a pull request.
func DefaultBlockedGHAPIEndpoints() []string {
	return []string{
		"PUT **/contents/**",
		"DELETE **/contents/**",
		"POST **/git/commits",
		"POST **/merges",
		"PUT **/pulls/*/merge",
	}
}

// DefaultBlockedGHAPIMutations returns the GraphQL mutations that create a commit.
func DefaultBlockedGHAPIMutations() []string {
	return []string{"createCommitOnBranch"}
}

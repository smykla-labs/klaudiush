package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/smykla-skalski/klaudiush/internal/rules"
	"github.com/smykla-skalski/klaudiush/internal/validator"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
	"github.com/smykla-skalski/klaudiush/pkg/parser"
)

const (
	apiValidatorName = "validate-gh-api"

	// methodWildcard matches any HTTP method in a blocked endpoint rule.
	methodWildcard = "*"

	// graphqlPath is the normalized endpoint of a GraphQL request.
	graphqlPath = "graphql"

	// bypassExplanation names what a commit made this way skips.
	bypassExplanation = "bypassing commit validation (GPG signing, sign-off, conventional commit format)"

	// apiHelp names the intended path, so the refusal is not just a "no".
	apiHelp = "Clone the repository, stage the change, and commit with git commit -sS. " +
		"gh api calls that create no commit are unaffected: reads, " +
		"POST /repos/{owner}/{repo}/git/refs, gh pr create."

	// unverifiableHelp tells the caller how to make the call checkable.
	unverifiableHelp = "Spell the endpoint literally in the command, or pass the GraphQL query " +
		"with -f query=... or on stdin, so klaudiush can tell whether it creates a commit."
)

// blockedEndpoint pairs an HTTP method with a compiled endpoint pattern.
type blockedEndpoint struct {
	method  string
	pattern rules.Pattern
}

// APIValidator rejects gh api calls that create a commit through the GitHub
// API. Such a commit never runs git, so no commit validator ever sees it.
type APIValidator struct {
	validator.BaseValidator
	config    *config.APIValidatorConfig
	endpoints []blockedEndpoint
	mutations []string
}

// NewAPIValidator creates a new APIValidator instance.
func NewAPIValidator(
	cfg *config.APIValidatorConfig,
	log logger.Logger,
	ruleAdapter validator.RuleChecker,
) *APIValidator {
	apiValidator := &APIValidator{
		BaseValidator: *validator.NewBaseValidatorWithRules(
			apiValidatorName, log, ruleAdapter,
		),
		config: cfg,
	}

	apiValidator.endpoints = compileBlockedEndpoints(
		apiValidator.blockedEndpointRules(),
		apiValidator.Logger(),
	)
	apiValidator.mutations = apiValidator.blockedMutations()

	return apiValidator
}

// blockedEndpointRules returns the configured endpoint rules, or the defaults.
func (v *APIValidator) blockedEndpointRules() []string {
	if v.config != nil && len(v.config.BlockedEndpoints) > 0 {
		return v.config.BlockedEndpoints
	}

	return config.DefaultBlockedGHAPIEndpoints()
}

// blockedMutations returns the configured GraphQL mutations, or the defaults.
func (v *APIValidator) blockedMutations() []string {
	if v.config != nil && len(v.config.BlockedGraphQLMutations) > 0 {
		return v.config.BlockedGraphQLMutations
	}

	return config.DefaultBlockedGHAPIMutations()
}

// blocksUnverifiable reports whether a write whose endpoint or GraphQL body
// cannot be read is rejected. Default: true, since an opaque write to the
// GitHub API cannot be shown to be safe.
func (v *APIValidator) blocksUnverifiable() bool {
	if v.config != nil && v.config.BlockUnverifiableCalls != nil {
		return *v.config.BlockUnverifiableCalls
	}

	return true
}

// compileBlockedEndpoints turns "METHOD pattern" rules into matchers. A rule
// that does not compile is skipped rather than failing the whole hook.
func compileBlockedEndpoints(specs []string, log logger.Logger) []blockedEndpoint {
	blocked := make([]blockedEndpoint, 0, len(specs))

	for _, spec := range specs {
		method, patternStr, found := strings.Cut(strings.TrimSpace(spec), " ")
		if !found {
			log.Error("Ignoring gh api rule without a method", "rule", spec)

			continue
		}

		pattern, err := rules.CompilePattern(strings.TrimSpace(patternStr))
		if err != nil {
			log.Error("Ignoring gh api rule with an invalid pattern", "rule", spec, "error", err)

			continue
		}

		blocked = append(blocked, blockedEndpoint{
			method:  strings.ToUpper(method),
			pattern: pattern,
		})
	}

	return blocked
}

// Validate rejects gh api invocations that would create a commit.
func (v *APIValidator) Validate(ctx context.Context, hookCtx *hook.Context) *validator.Result {
	log := v.Logger()
	log.Debug("Running gh api validation")

	if result := v.CheckRules(ctx, hookCtx); result != nil {
		return result
	}

	bashParser := parser.NewBashParser()

	parsed, err := bashParser.Parse(hookCtx.GetCommand())
	if err != nil {
		log.Error("Failed to parse command", "error", err)

		return validator.Warn(fmt.Sprintf("Failed to parse command: %v", err))
	}

	for _, cmd := range parsed.Commands {
		if !parser.IsGHAPI(&cmd) {
			continue
		}

		apiCmd, parseErr := parser.ParseGHAPICommand(cmd)
		if parseErr != nil {
			log.Debug("Skipping unparseable gh api command", "error", parseErr)

			continue
		}

		if result := v.checkAPICommand(parsed, apiCmd); result != nil {
			return result
		}
	}

	log.Debug("No commit-creating gh api commands found")

	return validator.Pass()
}

// checkAPICommand returns a failure when the call creates a commit, nil otherwise.
func (v *APIValidator) checkAPICommand(
	parsed *parser.ParseResult,
	apiCmd *parser.GHAPICommand,
) *validator.Result {
	endpoint := parsed.ExpandVars(apiCmd.Endpoint)

	if apiCmd.IsGraphQL || endpoint == graphqlPath {
		return v.checkGraphQL(parsed, apiCmd)
	}

	return v.checkREST(apiCmd, endpoint)
}

// checkREST matches the request against the blocked endpoint rules.
func (v *APIValidator) checkREST(
	apiCmd *parser.GHAPICommand,
	endpoint string,
) *validator.Result {
	for _, blocked := range v.endpoints {
		if blocked.method != methodWildcard && blocked.method != apiCmd.Method {
			continue
		}

		if !blocked.pattern.Match(endpoint) {
			continue
		}

		return v.fail(fmt.Sprintf(
			"gh api %s %s creates a commit through the GitHub API, %s",
			apiCmd.Method, endpoint, bypassExplanation,
		))
	}

	if v.isOpaqueWrite(apiCmd, endpoint) {
		return v.failUnverifiable(fmt.Sprintf(
			"gh api %s %s cannot be checked: the endpoint is not spelled literally, "+
				"so there is no way to tell whether it creates a commit",
			apiCmd.Method, describeEndpoint(endpoint),
		))
	}

	return nil
}

// isOpaqueWrite reports whether the call changes server state through an
// endpoint that could not be resolved to a literal path.
func (v *APIValidator) isOpaqueWrite(apiCmd *parser.GHAPICommand, endpoint string) bool {
	if !v.blocksUnverifiable() || !apiCmd.IsWriteMethod() {
		return false
	}

	return endpoint == "" || parser.HasUnresolvedVars(endpoint)
}

// describeEndpoint renders an endpoint for a message, naming the empty case.
func describeEndpoint(endpoint string) string {
	if endpoint == "" {
		return "(no endpoint in the command)"
	}

	return endpoint
}

// checkGraphQL matches the mutation name inside the query body, since the path
// is always /graphql and carries no signal.
func (v *APIValidator) checkGraphQL(
	parsed *parser.ParseResult,
	apiCmd *parser.GHAPICommand,
) *validator.Result {
	query := apiCmd.Query

	if apiCmd.InputFile != "" {
		body, ok := v.readInputFile(parsed, apiCmd)
		if !ok {
			if !v.blocksUnverifiable() {
				return nil
			}

			return v.failUnverifiable(fmt.Sprintf(
				"gh api graphql reads its query from %s, which cannot be read, "+
					"so there is no way to tell whether it creates a commit",
				apiCmd.InputFile,
			))
		}

		query += "\n" + body
	}

	for _, mutation := range v.mutations {
		if !strings.Contains(query, mutation) {
			continue
		}

		return v.fail(fmt.Sprintf(
			"gh api graphql calls the %s mutation, which creates a commit through the GitHub API, %s",
			mutation,
			bypassExplanation,
		))
	}

	return nil
}

// readInputFile returns the --input body, preferring content written earlier in
// the same command line over what is on disk, since a file created by a heredoc
// does not exist yet when the PreToolUse hook runs.
func (v *APIValidator) readInputFile(
	parsed *parser.ParseResult,
	apiCmd *parser.GHAPICommand,
) (string, bool) {
	if content, ok := parsed.InlineFileContent(
		apiCmd.InputFile, apiCmd.WorkingDirectory, apiCmd.Location,
	); ok {
		return content, true
	}

	path := apiCmd.InputFile
	if !filepath.IsAbs(path) && apiCmd.WorkingDirectory != "" {
		path = filepath.Join(apiCmd.WorkingDirectory, path)
	}

	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		v.Logger().Debug("Cannot read gh api input file", "path", path, "error", err)

		return "", false
	}

	return string(content), true
}

// fail builds the blocking result. FixHint comes from the suggestions registry.
func (*APIValidator) fail(message string) *validator.Result {
	return validator.FailWithRef(validator.RefGHAPICommit, message).
		AddDetail("help", apiHelp)
}

// failUnverifiable blocks a write that cannot be shown to be safe.
func (*APIValidator) failUnverifiable(message string) *validator.Result {
	return validator.FailWithRef(validator.RefGHAPIUnverifiable, message).
		AddDetail("help", unverifiableHelp)
}

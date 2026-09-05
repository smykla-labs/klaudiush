package github

import (
	"context"
	"fmt"
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

	// bypassExplanation names what a commit made this way skips.
	bypassExplanation = "bypassing commit validation (GPG signing, sign-off, conventional commit format)"

	// apiHelp names the intended path, so the refusal is not just a "no".
	apiHelp = "Clone the repository, stage the change, and commit with git commit -sS. " +
		"gh api calls that create no commit are unaffected: reads, " +
		"POST /repos/{owner}/{repo}/git/refs, gh pr create."
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

		if result := v.checkAPICommand(apiCmd); result != nil {
			return result
		}
	}

	log.Debug("No commit-creating gh api commands found")

	return validator.Pass()
}

// checkAPICommand returns a failure when the call creates a commit, nil otherwise.
func (v *APIValidator) checkAPICommand(apiCmd *parser.GHAPICommand) *validator.Result {
	if apiCmd.IsGraphQL {
		return v.checkGraphQL(apiCmd)
	}

	return v.checkREST(apiCmd)
}

// checkREST matches the request against the blocked endpoint rules.
func (v *APIValidator) checkREST(apiCmd *parser.GHAPICommand) *validator.Result {
	if apiCmd.Endpoint == "" {
		return nil
	}

	for _, blocked := range v.endpoints {
		if blocked.method != methodWildcard && blocked.method != apiCmd.Method {
			continue
		}

		if !blocked.pattern.Match(apiCmd.Endpoint) {
			continue
		}

		return v.fail(fmt.Sprintf(
			"gh api %s %s creates a commit through the GitHub API, %s",
			apiCmd.Method, apiCmd.Endpoint, bypassExplanation,
		))
	}

	return nil
}

// checkGraphQL matches the mutation name inside the query body, since the path
// is always /graphql and carries no signal.
func (v *APIValidator) checkGraphQL(apiCmd *parser.GHAPICommand) *validator.Result {
	for _, mutation := range v.mutations {
		if !strings.Contains(apiCmd.Query, mutation) {
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

// fail builds the blocking result. FixHint comes from the suggestions registry.
func (*APIValidator) fail(message string) *validator.Result {
	return validator.FailWithRef(validator.RefGHAPICommit, message).
		AddDetail("help", apiHelp)
}

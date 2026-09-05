package github

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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

	// reposPrefix is where every commit-creating REST endpoint lives.
	reposPrefix = "repos/"

	// ghesRESTPrefix and ghesGraphQLPrefix identify a GitHub Enterprise Server
	// API path, which can appear on any hostname.
	ghesRESTPrefix    = "api/v3/"
	ghesGraphQLPrefix = "api/graphql"

	// maxRequestBodyBytes caps how much of a request body file is read.
	maxRequestBodyBytes = 1 << 20

	// bypassExplanation names what a commit made this way skips.
	bypassExplanation = "bypassing commit validation (GPG signing, sign-off, conventional commit format)"

	// apiHelp names the intended path, so the refusal is not just a "no".
	apiHelp = "Clone the repository, stage the change, and commit with git commit -sS. " +
		"API calls that create no commit are unaffected: reads, " +
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

// checksHTTPClients reports whether clients other than gh are inspected.
func (v *APIValidator) checksHTTPClients() bool {
	if v.config != nil && v.config.CheckHTTPClients != nil {
		return *v.config.CheckHTTPClients
	}

	return true
}

// hosts returns the hostnames treated as the GitHub API.
func (v *APIValidator) hosts() []string {
	if v.config != nil && len(v.config.Hosts) > 0 {
		return v.config.Hosts
	}

	return config.DefaultGitHubAPIHosts()
}

// blockedClientCalls returns the library method names that create a commit.
func (v *APIValidator) blockedClientCalls() []string {
	if v.config != nil && len(v.config.BlockedClientCalls) > 0 {
		return v.config.BlockedClientCalls
	}

	return config.DefaultBlockedGHAPIClientCalls()
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
		if result := v.checkCommand(parsed, cmd); result != nil {
			return result
		}
	}

	if result := v.checkScriptText(hookCtx.GetCommand()); result != nil {
		return result
	}

	log.Debug("No commit-creating API calls found")

	return validator.Pass()
}

// checkCommand dispatches one parsed command to the matching request source.
func (v *APIValidator) checkCommand(
	parsed *parser.ParseResult,
	cmd parser.Command,
) *validator.Result {
	switch {
	case parser.IsGHAPI(&cmd):
		apiCmd, err := parser.ParseGHAPICommand(cmd)
		if err != nil {
			v.Logger().Debug("Skipping unparseable gh api command", "error", err)

			return nil
		}

		return v.checkAPICommand(parsed, apiCmd)

	case v.checksHTTPClients() && parser.IsHTTPClient(&cmd):
		for _, req := range parser.ParseHTTPClientCommands(cmd) {
			if result := v.checkHTTPRequest(parsed, req); result != nil {
				return result
			}
		}

		return nil

	default:
		return nil
	}
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
	if v.blocksEndpoint(apiCmd.Method, endpoint) {
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
		body, ok := v.readFile(
			parsed, apiCmd.InputFile, apiCmd.WorkingDirectory, apiCmd.Location,
		)
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

	if mutation := v.matchMutation(query); mutation != "" {
		return v.fail(fmt.Sprintf(
			"gh api graphql calls the %s mutation, which creates a commit through the GitHub API, %s",
			mutation,
			bypassExplanation,
		))
	}

	return nil
}

// readFile returns a request body from a file, preferring content written
// earlier in the same command line over what is on disk, since a file created
// by a heredoc does not exist yet when the PreToolUse hook runs.
func (v *APIValidator) readFile(
	parsed *parser.ParseResult,
	filePath, workDir string,
	location parser.Location,
) (string, bool) {
	if content, ok := parsed.InlineFileContent(filePath, workDir, location); ok {
		return content, true
	}

	path := filePath
	if !filepath.IsAbs(path) && workDir != "" {
		path = filepath.Join(workDir, path)
	}

	return v.readBodyFile(filepath.Clean(path))
}

// readBodyFile reads at most maxRequestBodyBytes of a regular file. The path
// comes from the command being validated, so an endless character device such
// as /dev/zero must not be read to the end, and a body far larger than any
// GraphQL document carries nothing worth matching.
func (v *APIValidator) readBodyFile(path string) (string, bool) {
	log := v.Logger()

	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		log.Debug("Skipping API request body file", "path", path, "error", err)

		return "", false
	}

	//nolint:gosec // path comes from the tool invocation klaudiush is validating
	file, err := os.Open(path)
	if err != nil {
		log.Debug("Cannot open API request body file", "path", path, "error", err)

		return "", false
	}

	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(io.LimitReader(file, maxRequestBodyBytes))
	if err != nil {
		log.Debug("Cannot read API request body file", "path", path, "error", err)

		return "", false
	}

	return string(content), true
}

// checkHTTPRequest applies the same endpoint rules to a curl, wget, httpie or
// xh call, once its URL is shown to address the GitHub API.
func (v *APIValidator) checkHTTPRequest(
	parsed *parser.ParseResult,
	req *parser.HTTPRequest,
) *validator.Result {
	host, path := parser.SplitRequestURL(parsed.ExpandVars(req.URL))
	if !v.isGitHubAPI(host, path) {
		return nil
	}

	endpoint := parser.NormalizeAPIEndpoint(path)

	if endpoint == graphqlPath {
		return v.checkRequestBody(parsed, req)
	}

	if v.blocksEndpoint(req.Method, endpoint) {
		return v.fail(fmt.Sprintf(
			"%s %s %s creates a commit through the GitHub API, %s",
			req.Tool, req.Method, endpoint, bypassExplanation,
		))
	}

	return nil
}

// checkRequestBody looks for a commit-creating mutation in a GraphQL body sent
// by a client other than gh.
func (v *APIValidator) checkRequestBody(
	parsed *parser.ParseResult,
	req *parser.HTTPRequest,
) *validator.Result {
	body := req.Body

	if req.BodyFile != "" {
		if content, ok := v.readFile(parsed, req.BodyFile, req.WorkingDirectory, req.Location); ok {
			body += "\n" + content
		}
	}

	if mutation := v.matchMutation(body); mutation != "" {
		return v.fail(fmt.Sprintf(
			"%s sends the %s mutation to the GitHub GraphQL API, "+
				"which creates a commit, %s",
			req.Tool, mutation, bypassExplanation,
		))
	}

	return nil
}

// checkScriptText looks for API calls written inside an inline script body, so
// a request made from node -e or python -c is seen even though the command
// itself is only an interpreter invocation.
func (v *APIValidator) checkScriptText(command string) *validator.Result {
	if !v.checksHTTPClients() {
		return nil
	}

	if call := parser.FindCallsInText(command, v.blockedClientCalls()); call != "" {
		return v.fail(fmt.Sprintf(
			"the script calls %s, which creates a commit through the GitHub API, %s",
			call, bypassExplanation,
		))
	}

	for _, req := range parser.FindAPICallsInText(command) {
		host, path := parser.SplitURL(req.URL)
		endpoint := parser.NormalizeAPIEndpoint(path)

		if host != "" {
			if !v.isGitHubAPI(host, path) {
				continue
			}
		} else if !req.ExplicitAPICall || !strings.HasPrefix(endpoint, reposPrefix) {
			// A path with no host is only a GitHub call when the syntax names
			// an API client and the path sits under repos/, where every
			// commit-creating REST endpoint lives. Anything else is as likely
			// to be a local route in the same script.
			continue
		}

		if v.blocksEndpoint(req.Method, endpoint) {
			return v.fail(fmt.Sprintf(
				"the script sends %s %s, which creates a commit through the GitHub API, %s",
				req.Method, endpoint, bypassExplanation,
			))
		}
	}

	return nil
}

// isGitHubAPI reports whether a host and path address the GitHub API. A path
// under the Enterprise Server prefixes counts on any host.
func (v *APIValidator) isGitHubAPI(host, path string) bool {
	if slices.Contains(v.hosts(), host) {
		return true
	}

	return strings.HasPrefix(path, "/"+ghesRESTPrefix) ||
		strings.HasPrefix(path, "/"+ghesGraphQLPrefix)
}

// blocksEndpoint reports whether a rule rejects this method and endpoint.
func (v *APIValidator) blocksEndpoint(method, endpoint string) bool {
	for _, blocked := range v.endpoints {
		if blocked.method != methodWildcard && blocked.method != method {
			continue
		}

		if blocked.pattern.Match(endpoint) {
			return true
		}
	}

	return false
}

// matchMutation returns the blocked mutation found in a GraphQL body.
func (v *APIValidator) matchMutation(body string) string {
	for _, mutation := range v.mutations {
		if strings.Contains(body, mutation) {
			return mutation
		}
	}

	return ""
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

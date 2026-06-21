package git

import (
	"context"
	"slices"
	"strings"

	"github.com/smykla-skalski/klaudiush/internal/templates"
	"github.com/smykla-skalski/klaudiush/internal/validator"
	"github.com/smykla-skalski/klaudiush/pkg/config"
	"github.com/smykla-skalski/klaudiush/pkg/hook"
	"github.com/smykla-skalski/klaudiush/pkg/logger"
	"github.com/smykla-skalski/klaudiush/pkg/parser"
)

const (
	defaultRemote = "origin"
)

// PushValidator validates git push commands
type PushValidator struct {
	validator.BaseValidator
	gitRunner GitRunner
	config    *config.PushValidatorConfig
}

// NewPushValidator creates a new PushValidator instance
func NewPushValidator(
	log logger.Logger,
	gitRunner GitRunner,
	cfg *config.PushValidatorConfig,
	ruleAdapter validator.RuleChecker,
) *PushValidator {
	return &PushValidator{
		BaseValidator: *validator.NewBaseValidatorWithRules(
			"validate-git-push", log, ruleAdapter,
		),
		gitRunner: defaultGitRunner(gitRunner),
		config:    cfg,
	}
}

// Name returns the validator name
func (*PushValidator) Name() string {
	return "validate-git-push"
}

// Validate validates git push commands
func (v *PushValidator) Validate(
	ctx context.Context,
	hookCtx *hook.Context,
) *validator.Result {
	if result := v.CheckRules(ctx, hookCtx); result != nil {
		return result
	}

	return ValidateGitSubcommand(
		ctx,
		hookCtx,
		v.Logger(),
		"push",
		v.validatePushCommand,
	)
}

// validatePushCommand validates a single git push command
func (v *PushValidator) validatePushCommand(
	gitCmd *parser.GitCommand,
	pendingRemotes map[string]bool,
) *validator.Result {
	log := v.Logger()

	// Use path-specific runner if -C flag is present
	runner := v.getRunnerForCommand(gitCmd)

	if !runner.IsInRepo() {
		log.Debug("not in a git repository, skipping validation")
		return validator.Pass()
	}

	remote := v.extractRemote(gitCmd, runner)
	if remote == "" {
		log.Debug("no remote specified, skipping validation")
		return validator.Pass()
	}

	// A bare variable remote (e.g. "git push $REMOTE main") is an unresolved
	// expansion whose runtime value the hook cannot see, so neither the blocked
	// list nor existence can be checked meaningfully. Skip rather than block.
	if isBareExpansion(remote) {
		log.Debug("remote is an unresolved variable; skipping validation", "remote", remote)

		return validator.Pass()
	}

	// Check if remote is blocked (before checking if it exists)
	if result := v.validateNotBlockedRemote(remote, runner); !result.Passed {
		return result
	}

	// Check if branch is blocked
	if v.config != nil && len(v.config.BlockedBranches) > 0 {
		branch := v.extractBranch(gitCmd, runner)
		if result := v.validateNotBlockedBranch(branch); !result.Passed {
			return result
		}
	}

	// Skip remote existence check if a preceding command adds this remote
	if pendingRemotes[remote] {
		v.Logger().
			Debug("remote being added by preceding command, skipping check", "remote", remote)

		return validator.Pass()
	}

	return v.validateRemoteExists(remote, runner)
}

// getRunnerForCommand returns the appropriate git runner for the command.
// If the command specifies a working directory with -C, creates a runner for that path.
// Otherwise, returns the default cached runner.
//

func (v *PushValidator) getRunnerForCommand(gitCmd *parser.GitCommand) GitRunner {
	workDir := gitCmd.GetWorkingDirectory()
	if workDir != "" {
		v.Logger().Debug("using path-specific runner", "path", workDir)
		return NewGitRunnerForPath(workDir)
	}

	return v.gitRunner
}

// extractRemote extracts the remote name from a git push command
func (*PushValidator) extractRemote(gitCmd *parser.GitCommand, runner GitRunner) string {
	if len(gitCmd.Args) == 0 {
		branch, err := runner.GetCurrentBranch()
		if err != nil {
			return defaultRemote
		}

		remote, err := runner.GetBranchRemote(branch)
		if err != nil {
			return defaultRemote
		}

		return remote
	}

	for _, arg := range gitCmd.Args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}

	return ""
}

// validateNotBlockedRemote checks if the remote is blocked
func (v *PushValidator) validateNotBlockedRemote(
	remote string,
	runner GitRunner,
) *validator.Result {
	// No config or no blocked remotes means all remotes are allowed
	if v.config == nil || len(v.config.BlockedRemotes) == 0 {
		return validator.Pass()
	}

	// Check if remote is in blocked list
	if !slices.Contains(v.config.BlockedRemotes, remote) {
		return validator.Pass()
	}

	// Format blocked remotes as comma-separated string
	blockedRemotesStr := strings.Join(v.config.BlockedRemotes, ", ")

	// Remote is blocked - get all available remotes
	allRemotes, err := runner.GetRemotes()
	if err != nil {
		// If we can't get remotes, just show blocked list without suggestions
		return validator.FailWithRef(
			validator.RefGitBlockedRemote,
			templates.MustExecute(
				templates.PushBlockedRemoteTemplate,
				templates.PushBlockedRemoteData{
					Remote:            remote,
					BlockedRemotesStr: blockedRemotesStr,
				},
			),
		)
	}

	// Get allowed remote priority list (default: ["origin", "upstream"])
	allowedPriority := v.config.AllowedRemotePriority
	if len(allowedPriority) == 0 {
		allowedPriority = []string{"origin", "upstream"}
	}

	// Find suggested remotes based on priority list
	var suggestedRemoteNames []string

	for _, priorityRemote := range allowedPriority {
		if _, exists := allRemotes[priorityRemote]; exists {
			// Don't suggest blocked remotes
			if !slices.Contains(v.config.BlockedRemotes, priorityRemote) {
				suggestedRemoteNames = append(suggestedRemoteNames, priorityRemote)
			}
		}
	}

	// If no suggested remotes from priority list, show all available remotes
	var availableRemoteNames []string

	if len(suggestedRemoteNames) == 0 {
		for name := range allRemotes {
			// Don't show blocked remotes
			if !slices.Contains(v.config.BlockedRemotes, name) {
				availableRemoteNames = append(availableRemoteNames, name)
			}
		}
	}

	result := validator.FailWithRef(
		validator.RefGitBlockedRemote,
		templates.MustExecute(
			templates.PushBlockedRemoteTemplate,
			templates.PushBlockedRemoteData{
				Remote:              remote,
				BlockedRemotesStr:   blockedRemotesStr,
				SuggestedRemotesStr: strings.Join(suggestedRemoteNames, ", "),
				AvailableRemotesStr: strings.Join(availableRemoteNames, ", "),
			},
		),
	)

	if len(suggestedRemoteNames) > 0 {
		result = result.WithFixHint(
			"Push to '" + suggestedRemoteNames[0] + "' instead: git push " + suggestedRemoteNames[0] + " <branch>",
		)
	}

	return result
}

// parseBranchFromRefspec extracts and normalizes the target branch from a refspec.
func parseBranchFromRefspec(refspec string) string {
	branch := refspec
	if _, dst, ok := strings.Cut(refspec, ":"); ok {
		branch = dst
	}

	return strings.TrimPrefix(branch, "refs/heads/")
}

// extractBranch extracts the target branch name from a git push command.
// When multiple refspecs are provided, returns a blocked branch first
// so downstream validation can deny the push.
func (v *PushValidator) extractBranch(gitCmd *parser.GitCommand, runner GitRunner) string {
	if len(gitCmd.Args) <= 1 {
		branch, err := runner.GetCurrentBranch()
		if err != nil {
			return ""
		}

		return branch
	}

	var firstBranch string

	for _, refspec := range gitCmd.Args[1:] {
		branch := parseBranchFromRefspec(refspec)
		if branch == "" {
			continue
		}

		if firstBranch == "" {
			firstBranch = branch
		}

		if v.config != nil && slices.Contains(v.config.BlockedBranches, branch) {
			return branch
		}
	}

	return firstBranch
}

// validateNotBlockedBranch checks if the branch is blocked
func (v *PushValidator) validateNotBlockedBranch(branch string) *validator.Result {
	if v.config == nil || len(v.config.BlockedBranches) == 0 || branch == "" {
		return validator.Pass()
	}

	if !slices.Contains(v.config.BlockedBranches, branch) {
		return validator.Pass()
	}

	blockedBranchesStr := strings.Join(v.config.BlockedBranches, ", ")

	return validator.FailWithRef(
		validator.RefGitBlockedBranch,
		templates.MustExecute(
			templates.PushBlockedBranchTemplate,
			templates.PushBlockedBranchData{
				Branch:             branch,
				BlockedBranchesStr: blockedBranchesStr,
			},
		),
	)
}

// validateRemoteExists checks if the remote exists
func (*PushValidator) validateRemoteExists(remote string, runner GitRunner) *validator.Result {
	helper := NewRemoteHelper()

	return helper.ValidateRemoteExists(
		remote,
		runner,
		validator.RefGitNoRemote,
	)
}

// Category returns the validator category for parallel execution.
// PushValidator uses CategoryGit because it queries git remote and branch state.
func (*PushValidator) Category() validator.ValidatorCategory {
	return validator.CategoryGit
}

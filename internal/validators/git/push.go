package git

import (
	"context"
	"strings"

	"github.com/smykla-labs/klaudiush/internal/rules"
	"github.com/smykla-labs/klaudiush/internal/validator"
	"github.com/smykla-labs/klaudiush/pkg/config"
	"github.com/smykla-labs/klaudiush/pkg/hook"
	"github.com/smykla-labs/klaudiush/pkg/logger"
	"github.com/smykla-labs/klaudiush/pkg/parser"
)

const (
	defaultRemote = "origin"
)

// PushValidator validates git push commands
type PushValidator struct {
	validator.BaseValidator
	gitRunner   GitRunner
	config      *config.PushValidatorConfig
	ruleAdapter *rules.RuleValidatorAdapter
}

// NewPushValidator creates a new PushValidator instance
func NewPushValidator(
	log logger.Logger,
	gitRunner GitRunner,
	cfg *config.PushValidatorConfig,
	ruleAdapter *rules.RuleValidatorAdapter,
) *PushValidator {
	if gitRunner == nil {
		gitRunner = NewGitRunner()
	}

	return &PushValidator{
		BaseValidator: *validator.NewBaseValidator("validate-git-push", log),
		gitRunner:     gitRunner,
		config:        cfg,
		ruleAdapter:   ruleAdapter,
	}
}

// Name returns the validator name
func (*PushValidator) Name() string {
	return "validate-git-push"
}

// Validate validates git push commands
func (v *PushValidator) Validate(ctx context.Context, hookCtx *hook.Context) *validator.Result {
	return ValidateGitSubcommand(
		ctx,
		hookCtx,
		v.ruleAdapter,
		v.Logger(),
		"push",
		v.validatePushCommand,
	)
}

// validatePushCommand validates a single git push command
func (v *PushValidator) validatePushCommand(gitCmd *parser.GitCommand) *validator.Result {
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

	return v.validateRemoteExists(remote, runner)
}

// getRunnerForCommand returns the appropriate git runner for the command.
// If the command specifies a working directory with -C, creates a runner for that path.
// Otherwise, returns the default cached runner.
//
//nolint:ireturn // Returns interface for flexibility between cached and path-specific runners
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

// validateRemoteExists checks if the remote exists
func (*PushValidator) validateRemoteExists(remote string, runner GitRunner) *validator.Result {
	helper := NewRemoteHelper()

	return helper.ValidateRemoteExists(
		remote,
		runner,
		validator.RefGitNoRemote,
		"🚫 Git push validation failed:",
	)
}

// Category returns the validator category for parallel execution.
// PushValidator uses CategoryGit because it queries git remote and branch state.
func (*PushValidator) Category() validator.ValidatorCategory {
	return validator.CategoryGit
}

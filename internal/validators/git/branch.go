package git

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/smykla-labs/claude-hooks/internal/templates"
	"github.com/smykla-labs/claude-hooks/internal/validator"
	"github.com/smykla-labs/claude-hooks/pkg/hook"
	"github.com/smykla-labs/claude-hooks/pkg/logger"
	"github.com/smykla-labs/claude-hooks/pkg/parser"
)

// BranchValidator validates git branch names.
type BranchValidator struct {
	validator.BaseValidator
}

// NewBranchValidator creates a new BranchValidator.
func NewBranchValidator(log logger.Logger) *BranchValidator {
	return &BranchValidator{
		BaseValidator: *validator.NewBaseValidator("validate-branch-name", log),
	}
}

const (
	// minBranchParts is the minimum number of parts in a valid branch name.
	minBranchParts = 2
)

var (
	// Valid branch name pattern: type/description (e.g., feat/add-feature, fix/bug-123).
	branchNamePattern = regexp.MustCompile(`^[a-z]+/[a-z0-9-]+$`)

	// Protected branches that should skip validation.
	protectedBranches = map[string]bool{
		"main":   true,
		"master": true,
	}

	// Valid branch types.
	validBranchTypes = map[string]bool{
		"feat":     true,
		"fix":      true,
		"docs":     true,
		"style":    true,
		"refactor": true,
		"test":     true,
		"chore":    true,
		"ci":       true,
		"build":    true,
		"perf":     true,
	}

	// Branch creation flags for git checkout.
	checkoutCreateFlags = []string{"-b", "--branch"}

	// Branch creation flags for git switch.
	switchCreateFlags = []string{"-c", "--create", "-C", "--force-create"}

	// Branch deletion flags for git branch.
	branchDeleteFlags = []string{"-d", "-D", "--delete"}
)

// Validate validates git branch names.
func (v *BranchValidator) Validate(_ context.Context, hookCtx *hook.Context) *validator.Result {
	log := v.Logger()
	log.Debug("validating git branch command")

	bashParser := parser.NewBashParser()

	parseResult, err := bashParser.Parse(hookCtx.ToolInput.Command)
	if err != nil {
		log.Error("failed to parse command", "error", err)
		return validator.Warn(fmt.Sprintf("Failed to parse command: %v", err))
	}

	for _, cmd := range parseResult.Commands {
		if cmd.Name != "git" {
			continue
		}

		gitCmd, err := parser.ParseGitCommand(cmd)
		if err != nil {
			v.Logger().Debug("failed to parse git command", "error", err)
			continue
		}

		result := v.validateGitCommand(gitCmd)
		if result != nil && !result.Passed {
			return result
		}
	}

	return validator.Pass()
}

// validateGitCommand validates a git command based on its subcommand.
func (v *BranchValidator) validateGitCommand(gitCmd *parser.GitCommand) *validator.Result {
	switch gitCmd.Subcommand {
	case "checkout":
		return v.validateCheckout(gitCmd)
	case "branch":
		return v.validateBranch(gitCmd)
	case "switch":
		return v.validateSwitch(gitCmd)
	default:
		return nil
	}
}

// validateCheckout validates git checkout -b/--branch commands that create new branches.
// Skips validation for commands without branch creation flags.
func (v *BranchValidator) validateCheckout(gitCmd *parser.GitCommand) *validator.Result {
	if !hasAnyFlag(gitCmd, checkoutCreateFlags) {
		return nil
	}

	return v.validateBranchCreation(gitCmd)
}

// validateBranch validates git branch commands that create new branches.
// Skips validation for delete operations.
func (v *BranchValidator) validateBranch(gitCmd *parser.GitCommand) *validator.Result {
	if hasAnyFlag(gitCmd, branchDeleteFlags) {
		return nil
	}

	return v.validateBranchCreation(gitCmd)
}

// validateSwitch validates git switch -c/--create/-C/--force-create commands that create new branches.
// Skips validation for commands without branch creation flags.
func (v *BranchValidator) validateSwitch(gitCmd *parser.GitCommand) *validator.Result {
	if !hasAnyFlag(gitCmd, switchCreateFlags) {
		return nil
	}

	return v.validateBranchCreation(gitCmd)
}

// validateBranchCreation performs the common validation logic for branch creation commands.
// Validates branch name format and checks for spaces.
func (v *BranchValidator) validateBranchCreation(gitCmd *parser.GitCommand) *validator.Result {
	branchName := v.extractBranchName(gitCmd)
	if branchName == "" {
		return nil
	}

	if strings.Contains(branchName, " ") {
		return v.createSpaceError()
	}

	return v.validateBranchName(branchName)
}

// createSpaceError creates an error for branch names with spaces.
func (*BranchValidator) createSpaceError() *validator.Result {
	message := templates.MustExecute(templates.BranchSpaceErrorTemplate, nil)
	return validator.Fail(message)
}

// extractBranchName extracts the branch name from a git command.
func (v *BranchValidator) extractBranchName(gitCmd *parser.GitCommand) string {
	switch gitCmd.Subcommand {
	case "checkout":
		return v.extractCheckoutBranchName(gitCmd)
	case "branch":
		return v.extractBranchCommandName(gitCmd)
	case "switch":
		return v.extractSwitchBranchName(gitCmd)
	default:
		return ""
	}
}

// extractCheckoutBranchName extracts the branch name from git checkout -b <branch> [start-point].
// The bash parser handles quoted strings, preserving spaces in a single argument.
func (*BranchValidator) extractCheckoutBranchName(gitCmd *parser.GitCommand) string {
	for _, flag := range checkoutCreateFlags {
		for i, f := range gitCmd.Flags {
			if f == flag && i+1 < len(gitCmd.Flags) {
				return gitCmd.Flags[i+1]
			}
		}
	}

	if len(gitCmd.Args) > 0 {
		return gitCmd.Args[0]
	}

	return ""
}

// extractBranchCommandName extracts the branch name from git branch <branch> [start-point].
// The bash parser handles quoted strings, preserving spaces in a single argument.
func (*BranchValidator) extractBranchCommandName(gitCmd *parser.GitCommand) string {
	if len(gitCmd.Args) > 0 {
		return gitCmd.Args[0]
	}

	return ""
}

// extractSwitchBranchName extracts the branch name from git switch -c <branch> [start-point].
// The bash parser handles quoted strings, preserving spaces in a single argument.
func (*BranchValidator) extractSwitchBranchName(gitCmd *parser.GitCommand) string {
	for _, flag := range switchCreateFlags {
		for i, f := range gitCmd.Flags {
			if f == flag && i+1 < len(gitCmd.Flags) {
				return gitCmd.Flags[i+1]
			}
		}
	}

	if len(gitCmd.Args) > 0 {
		return gitCmd.Args[0]
	}

	return ""
}

// hasAnyFlag checks if the git command has any of the flags in the provided list.
func hasAnyFlag(gitCmd *parser.GitCommand, flags []string) bool {
	return slices.ContainsFunc(flags, func(flag string) bool {
		return gitCmd.HasFlag(flag)
	})
}

// validateBranchName validates the branch name format (type/description).
// Skips validation for protected branches (main, master).
func (v *BranchValidator) validateBranchName(branchName string) *validator.Result {
	if protectedBranches[branchName] {
		v.Logger().Debug("skipping protected branch", "branch", branchName)
		return validator.Pass()
	}

	if branchName != strings.ToLower(branchName) {
		message := templates.MustExecute(
			templates.BranchUppercaseTemplate,
			templates.BranchUppercaseData{
				BranchName:  branchName,
				LowerBranch: strings.ToLower(branchName),
			},
		)

		return validator.Fail(message)
	}

	if !branchNamePattern.MatchString(branchName) {
		message := templates.MustExecute(
			templates.BranchPatternTemplate,
			templates.BranchPatternData{
				BranchName: branchName,
			},
		)

		return validator.Fail(message)
	}

	parts := strings.SplitN(branchName, "/", minBranchParts)
	if len(parts) != minBranchParts {
		message := templates.MustExecute(
			templates.BranchMissingPartsTemplate,
			templates.BranchMissingPartsData{
				BranchName: branchName,
			},
		)

		return validator.Fail(message)
	}

	branchType := parts[0]
	if !validBranchTypes[branchType] {
		validTypes := make([]string, 0, len(validBranchTypes))
		for t := range validBranchTypes {
			validTypes = append(validTypes, t)
		}

		message := templates.MustExecute(
			templates.BranchInvalidTypeTemplate,
			templates.BranchInvalidTypeData{
				BranchType:    branchType,
				ValidTypesStr: strings.Join(validTypes, ", "),
			},
		)

		return validator.Fail(message)
	}

	return validator.Pass()
}

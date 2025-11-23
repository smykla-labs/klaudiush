package git

import (
	"context"
	"strings"
	"time"

	"github.com/smykla-labs/claude-hooks/internal/exec"
)

// GitRunner defines the interface for git operations
type GitRunner interface {
	// IsInRepo checks if we're in a git repository
	IsInRepo() bool

	// GetStagedFiles returns the list of staged files
	GetStagedFiles() ([]string, error)

	// GetModifiedFiles returns the list of modified but unstaged files
	GetModifiedFiles() ([]string, error)

	// GetUntrackedFiles returns the list of untracked files
	GetUntrackedFiles() ([]string, error)

	// GetRepoRoot returns the git repository root directory
	GetRepoRoot() (string, error)

	// GetRemoteURL returns the URL for the given remote
	GetRemoteURL(remote string) (string, error)

	// GetCurrentBranch returns the current branch name
	GetCurrentBranch() (string, error)

	// GetBranchRemote returns the tracking remote for the given branch
	GetBranchRemote(branch string) (string, error)

	// GetRemotes returns the list of all remotes with their URLs
	GetRemotes() (map[string]string, error)
}

// CLIGitRunner implements GitRunner using actual git commands
type CLIGitRunner struct {
	runner  exec.CommandRunner
	timeout time.Duration
}

// NewCLIGitRunner creates a new CLIGitRunner instance
func NewCLIGitRunner() *CLIGitRunner {
	return &CLIGitRunner{
		runner:  exec.NewCommandRunner(gitCommandTimeout),
		timeout: gitCommandTimeout,
	}
}

// NewRealGitRunner creates a new CLIGitRunner instance
func NewRealGitRunner() *CLIGitRunner {
	return NewCLIGitRunner()
}

// IsInRepo checks if we're in a git repository
func (r *CLIGitRunner) IsInRepo() bool {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result := r.runner.Run(ctx, "git", "rev-parse", "--git-dir")

	return result.Err == nil
}

// GetStagedFiles returns the list of staged files
func (r *CLIGitRunner) GetStagedFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result := r.runner.Run(ctx, "git", "diff", "--cached", "--name-only")
	if result.Err != nil {
		return nil, result.Err
	}

	return parseLines(result.Stdout), nil
}

// GetModifiedFiles returns the list of modified but unstaged files
func (r *CLIGitRunner) GetModifiedFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result := r.runner.Run(ctx, "git", "diff", "--name-only")
	if result.Err != nil {
		return nil, result.Err
	}

	return parseLines(result.Stdout), nil
}

// GetUntrackedFiles returns the list of untracked files
func (r *CLIGitRunner) GetUntrackedFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result := r.runner.Run(ctx, "git", "ls-files", "--others", "--exclude-standard")
	if result.Err != nil {
		return nil, result.Err
	}

	return parseLines(result.Stdout), nil
}

// GetRepoRoot returns the git repository root directory
func (r *CLIGitRunner) GetRepoRoot() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result := r.runner.Run(ctx, "git", "rev-parse", "--show-toplevel")
	if result.Err != nil {
		return "", result.Err
	}

	return strings.TrimSpace(result.Stdout), nil
}

// GetRemoteURL returns the URL for the given remote
func (r *CLIGitRunner) GetRemoteURL(remote string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result := r.runner.Run(ctx, "git", "remote", "get-url", remote)
	if result.Err != nil {
		return "", result.Err
	}

	return strings.TrimSpace(result.Stdout), nil
}

// GetCurrentBranch returns the current branch name
func (r *CLIGitRunner) GetCurrentBranch() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result := r.runner.Run(ctx, "git", "symbolic-ref", "--short", "HEAD")
	if result.Err != nil {
		return "", result.Err
	}

	return strings.TrimSpace(result.Stdout), nil
}

// GetBranchRemote returns the tracking remote for the given branch
func (r *CLIGitRunner) GetBranchRemote(branch string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	configKey := "branch." + branch + ".remote"

	result := r.runner.Run(ctx, "git", "config", configKey)
	if result.Err != nil {
		return "", result.Err
	}

	return strings.TrimSpace(result.Stdout), nil
}

// GetRemotes returns the list of all remotes with their URLs
func (r *CLIGitRunner) GetRemotes() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	result := r.runner.Run(ctx, "git", "remote", "-v")
	if result.Err != nil {
		return nil, result.Err
	}

	remotes := make(map[string]string)

	lines := strings.SplitSeq(strings.TrimSpace(result.Stdout), "\n")
	for line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)

		const minFieldsRequired = 2

		if len(fields) >= minFieldsRequired {
			remoteName := fields[0]
			remoteURL := fields[1]
			// Only add each remote once (git remote -v shows fetch and push separately)
			if _, exists := remotes[remoteName]; !exists {
				remotes[remoteName] = remoteURL
			}
		}
	}

	return remotes, nil
}

// parseLines splits output by newlines and filters empty lines
func parseLines(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" {
		return []string{}
	}

	return strings.Split(output, "\n")
}

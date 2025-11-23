package git

import (
	gitvalidators "github.com/smykla-labs/claude-hooks/internal/validators/git"
)

// RepositoryAdapter adapts the Repository interface to implement GitRunner
// This provides backward compatibility with existing validators while using the SDK
type RepositoryAdapter struct {
	repo Repository
}

// NewRepositoryAdapter creates a new adapter that wraps a Repository
func NewRepositoryAdapter(repo Repository) *RepositoryAdapter {
	return &RepositoryAdapter{repo: repo}
}

// NewSDKGitRunner creates a GitRunner backed by the go-git SDK
//
//nolint:ireturn // Factory function returns interface by design
func NewSDKGitRunner() (gitvalidators.GitRunner, error) {
	repo, err := DiscoverRepository()
	if err != nil {
		return nil, err
	}

	return NewRepositoryAdapter(repo), nil
}

// IsInRepo checks if we're in a git repository
func (a *RepositoryAdapter) IsInRepo() bool {
	return a.repo.IsInRepo()
}

// GetStagedFiles returns the list of staged files
func (a *RepositoryAdapter) GetStagedFiles() ([]string, error) {
	return a.repo.GetStagedFiles()
}

// GetModifiedFiles returns the list of modified but unstaged files
func (a *RepositoryAdapter) GetModifiedFiles() ([]string, error) {
	return a.repo.GetModifiedFiles()
}

// GetUntrackedFiles returns the list of untracked files
func (a *RepositoryAdapter) GetUntrackedFiles() ([]string, error) {
	return a.repo.GetUntrackedFiles()
}

// GetRepoRoot returns the git repository root directory
func (a *RepositoryAdapter) GetRepoRoot() (string, error) {
	return a.repo.GetRoot()
}

// GetRemoteURL returns the URL for the given remote
func (a *RepositoryAdapter) GetRemoteURL(remote string) (string, error) {
	return a.repo.GetRemoteURL(remote)
}

// GetCurrentBranch returns the current branch name
func (a *RepositoryAdapter) GetCurrentBranch() (string, error) {
	return a.repo.GetCurrentBranch()
}

// GetBranchRemote returns the tracking remote for the given branch
func (a *RepositoryAdapter) GetBranchRemote(branch string) (string, error) {
	return a.repo.GetBranchRemote(branch)
}

// GetRemotes returns the list of all remotes with their URLs
func (a *RepositoryAdapter) GetRemotes() (map[string]string, error) {
	return a.repo.GetRemotes()
}

package git

// MockGitRunner implements GitRunner for testing without executing git commands
type MockGitRunner struct {
	InRepo         bool
	StagedFiles    []string
	ModifiedFiles  []string
	UntrackedFiles []string
	RepoRoot       string
	Remotes        map[string]string
	CurrentBranch  string
	BranchRemotes  map[string]string
	Err            error
}

// NewMockGitRunner creates a new MockGitRunner instance
func NewMockGitRunner() *MockGitRunner {
	return &MockGitRunner{
		InRepo:         true,
		StagedFiles:    []string{},
		ModifiedFiles:  []string{},
		UntrackedFiles: []string{},
		RepoRoot:       "/mock/repo",
		Remotes: map[string]string{
			"origin":   "git@github.com:user/repo.git",
			"upstream": "git@github.com:org/repo.git",
		},
		CurrentBranch: "main",
		BranchRemotes: map[string]string{
			"main": "origin",
		},
		Err: nil,
	}
}

// IsInRepo checks if we're in a git repository
func (m *MockGitRunner) IsInRepo() bool {
	return m.InRepo
}

// GetStagedFiles returns the list of staged files
func (m *MockGitRunner) GetStagedFiles() ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	return m.StagedFiles, nil
}

// GetModifiedFiles returns the list of modified but unstaged files
func (m *MockGitRunner) GetModifiedFiles() ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	return m.ModifiedFiles, nil
}

// GetUntrackedFiles returns the list of untracked files
func (m *MockGitRunner) GetUntrackedFiles() ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	return m.UntrackedFiles, nil
}

// GetRepoRoot returns the git repository root directory
func (m *MockGitRunner) GetRepoRoot() (string, error) {
	if m.Err != nil {
		return "", m.Err
	}

	return m.RepoRoot, nil
}

// GetRemoteURL returns the URL for the given remote
func (m *MockGitRunner) GetRemoteURL(remote string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}

	if url, ok := m.Remotes[remote]; ok {
		return url, nil
	}

	return "", &MockError{Msg: "remote not found"}
}

// GetCurrentBranch returns the current branch name
func (m *MockGitRunner) GetCurrentBranch() (string, error) {
	if m.Err != nil {
		return "", m.Err
	}

	return m.CurrentBranch, nil
}

// GetBranchRemote returns the tracking remote for the given branch
func (m *MockGitRunner) GetBranchRemote(branch string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}

	if remote, ok := m.BranchRemotes[branch]; ok {
		return remote, nil
	}

	return "", &MockError{Msg: "branch remote not found"}
}

// GetRemotes returns the list of all remotes with their URLs
func (m *MockGitRunner) GetRemotes() (map[string]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	return m.Remotes, nil
}

// MockError is a simple error type for testing
type MockError struct {
	Msg string
}

func (e *MockError) Error() string {
	return e.Msg
}

// Ensure MockGitRunner implements GitRunner
var _ GitRunner = (*MockGitRunner)(nil)

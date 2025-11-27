package git

import "github.com/cockroachdb/errors"

var (
	// ErrNotRepository is returned when the working directory is not a git repository
	ErrNotRepository = errors.New("not a git repository")

	// ErrNoHead is returned when the repository has no HEAD (e.g., empty repository)
	ErrNoHead = errors.New("repository has no HEAD")

	// ErrDetachedHead is returned when HEAD is detached
	ErrDetachedHead = errors.New("HEAD is detached")

	// ErrRemoteNotFound is returned when the specified remote does not exist
	ErrRemoteNotFound = errors.New("remote not found")

	// ErrBranchNotFound is returned when the specified branch does not exist
	ErrBranchNotFound = errors.New("branch not found")

	// ErrNoTracking is returned when a branch has no tracking configuration
	ErrNoTracking = errors.New("branch has no tracking remote")
)

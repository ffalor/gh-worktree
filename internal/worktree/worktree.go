package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ffalor/gh-worktree/internal/git"
)

// Create creates a new worktree.
// path: The absolute path where the worktree should be created.
// branch: The exact name of the branch to create.
// startPoint: The ref to start from (e.g., HEAD, FETCH_HEAD, an existing branch).
func Create(path, branch, startPoint string) error {
	var err error

	// Ensure the base directory exists
	baseDir := filepath.Dir(path)
	if err = os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Check if git still has a record of this worktree (even though it doesn't exist on disk)
	// and remove it if necessary
	if git.WorktreeIsRegistered(path) {
		if err = git.WorktreeRemove(path, true); err != nil {
			return fmt.Errorf("failed to remove stale worktree record: %w", err)
		}
	}

	if startPoint != "" {
		err = git.WorktreeAddFromRef(branch, path, startPoint)
	} else {
		err = git.WorktreeAdd(branch, path)
	}

	if err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	return nil
}

// Attach creates a new worktree from an existing branch.
func Attach(path, branch string) error {
	// Ensure the base directory exists
	baseDir := filepath.Dir(path)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	if err := git.WorktreeAddFromBranch(branch, path); err != nil {
		return fmt.Errorf("failed to attach to worktree: %w", err)
	}
	return nil
}

// Remove removes a worktree.
// This function is responsible for running `git worktree remove` and ensuring the directory is gone.
func Remove(path string, force bool) error {
	// Check for uncommitted changes if not forced
	if !force && git.HasUncommittedChanges(path) {
		return fmt.Errorf("worktree has uncommitted changes")
	}

	// Try to get the exact path from git's records
	var exactPath string
	worktrees, err := git.ListWorktrees()
	if err == nil {
		for _, wt := range worktrees {
			if strings.HasSuffix(wt, path) || wt == path {
				exactPath = wt
				break
			}
		}
	}

	// Remove worktree from git records
	if exactPath != "" {
		if err := git.WorktreeRemove(exactPath, force); err != nil {
			// If git worktree remove fails, try manual removal as a fallback
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
	}

	// Final cleanup: ensure the directory is removed, even if it wasn't registered in git
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return os.RemoveAll(path)
	}

	return nil
}

// Exists checks if a worktree already exists on disk.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

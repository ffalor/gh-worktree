package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gh "github.com/cli/go-gh/v2"
)

// Command runs a git command in the specified directory
func Command(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CommandSilent runs a git command without output
func CommandSilent(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

// CommandOutput runs a git command and returns the output
func CommandOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// CloneBare clones a repository as a bare repository using gh CLI
func CloneBare(dir, repo, dest string) error {
	dest = filepath.Join(dir, dest)
	args := []string{"repo", "clone", repo, dest, "--", "--bare"}
	_, stderr, err := gh.Exec(args...)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %s", stderr.String())
	}
	return nil
}

// ConfigRemote sets the remote fetch spec to include all refs
func ConfigRemote(repoPath string) error {
	return Command(repoPath, "config", "--add", "remote.origin.fetch", "refs/heads/*:refs/remotes/origin/*")
}

// WorktreeAdd adds a worktree with a new branch
func WorktreeAdd(repoPath, branch, worktreePath string) error {
	return Command(repoPath, "worktree", "add", "-b", branch, worktreePath)
}

// WorktreeAddFromRef adds a worktree from a specific ref
func WorktreeAddFromRef(repoPath, branch, worktreePath, ref string) error {
	return Command(repoPath, "worktree", "add", "-b", branch, worktreePath, ref)
}

// WorktreeAddFromBranch adds a worktree from an existing branch
func WorktreeAddFromBranch(repoPath, branch, worktreePath string) error {
	return Command(repoPath, "worktree", "add", worktreePath, branch)
}

// WorktreeRemove removes a worktree
func WorktreeRemove(repoPath, worktreePath string) error {
	return Command(repoPath, "worktree", "remove", worktreePath)
}

// Fetch fetches refs from origin
func Fetch(repoPath string, refs ...string) error {
	args := append([]string{"fetch", "origin"}, refs...)
	return Command(repoPath, args...)
}

// BranchDelete deletes a branch
func BranchDelete(repoPath, branch string, force bool) error {
	args := []string{"branch", "-d"}
	if force {
		args[1] = "-D"
	}
	args = append(args, branch)
	return Command(repoPath, args...)
}

// HasUncommittedChanges checks if a worktree has uncommitted changes
func HasUncommittedChanges(worktreePath string) bool {
	// Check for staged or unstaged changes
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// GetCurrentBranch returns the current branch name
func GetCurrentBranch(repoPath string) (string, error) {
	out, err := CommandOutput(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ListWorktrees lists all worktrees for a repository
func ListWorktrees(repoPath string) ([]string, error) {
	out, err := CommandOutput(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var worktrees []string
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			worktrees = append(worktrees, path)
		}
	}
	return worktrees, nil
}

// WorktreePrune prunes stale worktree records
func WorktreePrune(repoPath string) error {
	return CommandSilent(repoPath, "worktree", "prune")
}

// IsGitRepository checks if a directory is a git repository
func IsGitRepository(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	err := cmd.Run()
	return err == nil
}

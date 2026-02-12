package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gh "github.com/cli/go-gh/v2"
)

// Command runs a git command in the current directory
func Command(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CommandSilent runs a git command without output in the current directory
func CommandSilent(args ...string) error {
	cmd := exec.Command("git", args...)
	return cmd.Run()
}

// CommandOutput runs a git command and returns the output from current directory
func CommandOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// CommandOutputAt runs a git command and returns the output from specified directory
func CommandOutputAt(path string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = path
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
func ConfigRemote() error {
	return Command("config", "--add", "remote.origin.fetch", "refs/heads/*:refs/remotes/origin/*")
}

// WorktreeAdd adds a worktree with a new branch
func WorktreeAdd(branch, worktreePath string) error {
	return Command("worktree", "add", "-b", branch, worktreePath)
}

// WorktreeAddFromRef adds a worktree from a specific ref
func WorktreeAddFromRef(branch, worktreePath, ref string) error {
	return Command("worktree", "add", "-b", branch, worktreePath, ref)
}

// WorktreeAddFromBranch adds a worktree from an existing branch
func WorktreeAddFromBranch(branch, worktreePath string) error {
	return Command("worktree", "add", worktreePath, branch)
}

// WorktreeRemove removes a worktree
func WorktreeRemove(worktreePath string, force bool) error {
	args := []string{"worktree", "remove", worktreePath}
	if force {
		args = append(args, "--force")
	}
	return Command(args...)
}

// Fetch fetches refs from origin
func Fetch(refs ...string) error {
	args := append([]string{"fetch", "origin"}, refs...)
	return Command(args...)
}

// BranchDelete deletes a branch
func BranchDelete(branch string, force bool) error {
	args := []string{"branch", "-d"}
	if force {
		args[1] = "-D"
	}
	args = append(args, branch)
	return Command(args...)
}

// BranchExists checks if a branch exists in the repository
func BranchExists(branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	err := cmd.Run()
	return err == nil
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

// GetCurrentBranch returns the current branch name in the specified directory
func GetCurrentBranch(path string) (string, error) {
	out, err := CommandOutputAt(path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetCurrentBranchAtCwd returns the current branch name at current working directory
func GetCurrentBranchAtCwd() (string, error) {
	out, err := CommandOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ListWorktrees lists all worktrees for a repository
func ListWorktrees() ([]string, error) {
	out, err := CommandOutput("worktree", "list", "--porcelain")
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

// WorktreeIsRegistered checks if a worktree path is registered in git
func WorktreeIsRegistered(worktreePath string) bool {
	worktrees, err := ListWorktrees()
	if err != nil {
		return false
	}
	for _, wt := range worktrees {
		if wt == worktreePath {
			return true
		}
	}
	return false
}

// WorktreePrune prunes stale worktree records
func WorktreePrune() error {
	return CommandSilent("worktree", "prune")
}

// IsGitRepository checks if a directory is a git repository
func IsGitRepository(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	err := cmd.Run()
	return err == nil
}

// GetGitDir returns the path to the .git directory
func GetGitDir(path string) (string, error) {
	out, err := CommandOutput(path, "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func GetGitCommonDir(path string) (string, error) {
	out, err := CommandOutput(path, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func IsBareRepository(path string) bool {
	out, err := CommandOutput(path, "rev-parse", "--is-bare-repository")
	return err == nil && strings.TrimSpace(out) == "true"
}

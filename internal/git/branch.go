package git

import (
	"os/exec"
	"strings"
)

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

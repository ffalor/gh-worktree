package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cli/go-gh/v2/pkg/prompter"
	"github.com/ffalor/gh-worktree/internal/config"
	"github.com/ffalor/gh-worktree/internal/git"
	"github.com/ffalor/gh-worktree/internal/worktree"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove [worktree-name]",
	Short: "Remove a worktree and its associated branch",
	Long:  `Remove a worktree and its associated branch. Will prompt if there are uncommitted changes (unless --force is used).`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	worktreeName := args[0]
	baseDir := config.GetWorktreeBase()

	// Find the full path of the worktree to remove.
	worktreePath, err := findWorktreePath(baseDir, worktreeName)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Worktree '%s' not found, nothing to remove.\n", worktreeName)
			return nil // Not an error if it's already gone.
		}
		return err
	}

	// Get the branch name *before* removing the worktree.
	branch, err := git.GetCurrentBranch(worktreePath)
	if err != nil {
		// If we can't get the branch, we can still proceed with directory removal.
		// We'll just warn the user.
		fmt.Printf("Warning: could not determine branch for worktree %s. It may need to be manually removed. Error: %v\n", worktreeName, err)
		branch = "" // Clear branch so we don't try to delete it.
	}

	// Handle uncommitted changes prompt.
	force := forceFlag
	if !force && git.HasUncommittedChanges(worktreePath) {
		p := prompter.New(os.Stdin, os.Stdout, os.Stderr)
		confirm, err := p.Confirm("Worktree has uncommitted changes. Remove anyway?", false)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if !confirm {
			fmt.Println("Operation cancelled.")
			return nil
		}
		force = true // User confirmed.
	}

	// 1. Remove the worktree directory and git metadata.
	fmt.Printf("Removing worktree '%s'...\n", worktreeName)
	if err := worktree.Remove(worktreePath, force); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}
	fmt.Printf("Successfully removed worktree directory.\n")

	// 2. Delete the associated branch if we found one.
	if branch != "" {
		fmt.Printf("Deleting branch '%s'...\n", branch)
		if err := git.BranchDelete(branch, true); err != nil {
			// This is not a fatal error, as the primary goal (removing the worktree) succeeded.
			// The branch might be the main branch or have other worktrees, so git will prevent its deletion.
			return fmt.Errorf("worktree removed, but failed to delete branch '%s': %w. You may need to remove it manually", branch, err)
		}
		fmt.Printf("Successfully deleted branch '%s'.\n", branch)
	}

	fmt.Printf("\nWorktree '%s' and branch '%s' removed successfully.\n", worktreeName, branch)
	return nil
}

// findWorktreePath searches across all repo directories in the base directory
// for a worktree matching the given name.
func findWorktreePath(baseDir, name string) (string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("could not read worktree base directory '%s': %w", baseDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoDir := filepath.Join(baseDir, entry.Name())
		possiblePath := filepath.Join(repoDir, name)
		if _, err := os.Stat(possiblePath); err == nil {
			// Found it, return the full path
			return possiblePath, nil
		}
	}

	return "", os.ErrNotExist
}

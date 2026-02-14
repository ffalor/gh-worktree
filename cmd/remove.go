package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cli/go-gh/v2/pkg/prompter"
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

	// Require being in a git repository (consistent with create command)
	if !git.IsGitRepository(".") {
		return fmt.Errorf("not in a git repository")
	}

	// Find the worktree by name using git's worktree registry
	worktrees, err := git.GetWorktreeInfo()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Find all worktrees matching the given input (suffix matching like git worktree remove)
	var matches []git.WorktreeInfo
	for _, wt := range worktrees {
		if strings.HasSuffix(wt.Path, worktreeName) {
			matches = append(matches, wt)
		}
	}

	if len(matches) == 0 {
		fmt.Printf("Worktree '%s' not found in this repository.\n", worktreeName)
		return nil
	}

	// If multiple matches, prompt user to select one
	var targetWorktree git.WorktreeInfo
	if len(matches) == 1 {
		targetWorktree = matches[0]
	} else {
		options := make([]string, len(matches))
		for i, wt := range matches {
			options[i] = wt.Path
		}
		p := prompter.New(os.Stdin, os.Stdout, os.Stderr)
		idx, err := p.Select("Multiple worktrees match '"+worktreeName+"'. Select one:", "", options)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		targetWorktree = matches[idx]
	}

	// Handle uncommitted changes prompt.
	force := forceFlag
	if !force && git.HasUncommittedChanges(targetWorktree.Path) {
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
	fmt.Printf("Removing worktree '%s'...\n", targetWorktree.Path)
	if err := worktree.Remove(targetWorktree.Path, force); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}
	fmt.Printf("Successfully removed worktree directory.\n")

	// 2. Delete the associated branch if we found one.
	if targetWorktree.Branch != "" {
		fmt.Printf("Deleting branch '%s'...\n", targetWorktree.Branch)
		if err := git.BranchDelete(targetWorktree.Branch, true); err != nil {
			// This is not a fatal error, as the primary goal (removing the worktree) succeeded.
			// The branch might be the main branch or have other worktrees, so git will prevent its deletion.
			return fmt.Errorf("worktree removed, but failed to delete branch '%s': %w. You may need to remove it manually", targetWorktree.Branch, err)
		}
		fmt.Printf("Successfully deleted branch '%s'.\n", targetWorktree.Branch)
	}

	fmt.Printf("\nWorktree '%s' and branch '%s' removed successfully.\n", targetWorktree.Path, targetWorktree.Branch)
	return nil
}

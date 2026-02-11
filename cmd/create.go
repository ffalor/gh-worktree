package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/cli/go-gh/v2/pkg/prompter"
	"github.com/ffalor/gh-worktree/internal/config"
	"github.com/ffalor/gh-worktree/internal/git"
	"github.com/ffalor/gh-worktree/internal/worktree"
	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create [url|name]",
	Short: "Create a new worktree from a GitHub URL or branch name",
	Long: `Create a new git worktree from either:
- A GitHub pull request URL
- A GitHub issue URL
- A local branch name (when run from within a git repository)`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

var useExistingFlag bool

func init() {
	createCmd.Flags().BoolVarP(&useExistingFlag, "use-existing", "e", false, "use existing branch if it exists")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	arg := args[0]

	info, err := worktree.ParseArgument(arg)
	if err != nil {
		return err
	}

	baseDir := config.GetWorktreeBase()
	worktreePath := info.GetWorktreePath(baseDir)
	repoPath := info.GetRepoPath(baseDir)

	// Check if worktree directory already exists
	if worktree.WorktreeExists(worktreePath) {
		if forceFlag {
			// Force: remove existing worktree
			fmt.Printf("Force flag set. Removing existing worktree...\n")
			branch, _ := git.GetCurrentBranch(worktreePath)
			if err := worktree.Remove(repoPath, worktreePath, branch, true); err != nil {
				return fmt.Errorf("failed to remove existing worktree: %w", err)
			}
		} else {
			// Prompt user
			p := prompter.New(os.Stdin, os.Stdout, os.Stderr)
			confirm, err := p.Confirm(fmt.Sprintf("Worktree already exists at %s. Overwrite?", worktreePath), false)
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			if !confirm {
				fmt.Println("Operation cancelled")
				return nil
			}
			// Remove existing worktree
			branch, _ := git.GetCurrentBranch(worktreePath)
			if err := worktree.Remove(repoPath, worktreePath, branch, true); err != nil {
				return fmt.Errorf("failed to remove existing worktree: %w", err)
			}
		}
	}

	// Create worktree with cleanup on error
	creator := worktree.NewCreatorWithCheck(func(branchName string) worktree.BranchAction {
		// This callback is called after we know the branch name but before creating the worktree
		// It allows us to check for and handle orphaned branches
		if branchExists(repoPath, branchName) {
			if forceFlag {
				fmt.Printf("Removing existing branch '%s' from bare repository...\n", branchName)
				if err := git.BranchDelete(repoPath, branchName, true); err != nil {
					fmt.Printf("failed to delete existing branch: %v\n", err)
					return worktree.BranchActionCancel
				}
				return worktree.BranchActionOverwrite
			}

			if useExistingFlag {
				return worktree.BranchActionAttach
			}

			p := prompter.New(os.Stdin, os.Stdout, os.Stderr)
			options := []string{
				"Overwrite (delete and recreate)",
				"Attach (use existing branch)",
				"No (cancel)",
			}
			selectedIndex, err := p.Select(fmt.Sprintf("Branch '%s' already exists. What would you like to do?", branchName), "", options)
			if err != nil {
				fmt.Printf("prompt failed: %v\n", err)
				return worktree.BranchActionCancel
			}

			var selected worktree.BranchAction
			switch selectedIndex {
			case 0:
				selected = worktree.BranchActionOverwrite
			case 1:
				selected = worktree.BranchActionAttach
			case 2:
				selected = worktree.BranchActionCancel
			default:
				selected = worktree.BranchActionCancel
			}

			if selected == worktree.BranchActionOverwrite {
				fmt.Printf("Removing existing branch '%s' from bare repository...\n", branchName)
				if err := git.BranchDelete(repoPath, branchName, true); err != nil {
					fmt.Printf("failed to delete existing branch: %v\n", err)
					return worktree.BranchActionCancel
				}
			}
			return selected
		}
		return worktree.BranchActionOverwrite
	})

	defer func() {
		if r := recover(); r != nil {
			creator.Cleanup()
			panic(r)
		}
	}()

	err = creator.Create(info)
	if err != nil {
		creator.Cleanup()
		return err
	}

	return nil
}

// branchExists checks if a branch exists in the repository
func branchExists(repoPath, branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoPath
	err := cmd.Run()
	return err == nil
}

package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cli/go-gh/v2/pkg/prompter"
	"github.com/ffalor/gh-worktree/internal/config"
	"github.com/ffalor/gh-worktree/internal/git"
	"github.com/ffalor/gh-worktree/internal/worktree"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove [worktree-name|url]",
	Short: "Remove a worktree",
	Long:  `Remove a worktree and its associated branch. Will prompt if there are uncommitted changes (unless --force is used).`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	arg := args[0]

	var worktreeName, repoName, worktreePath string
	var info *worktree.WorktreeInfo
	var err error

	u, err := url.Parse(arg)
	if err == nil && u.Scheme == "https" && strings.Contains(arg, "github.com") {
		info, err = worktree.ParseArgument(arg)
		if err != nil {
			return err
		}
		repoName = info.Repo
		worktreeName = info.WorktreeName
		baseDir := config.GetWorktreeBase()
		worktreePath = info.GetWorktreePath(baseDir)
	} else {
		baseDir := config.GetWorktreeBase()
		repoName, worktreePath, err = findWorktree(baseDir, arg)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				fmt.Printf("Worktree '%s' not found, nothing to remove\n", arg)
				return nil
			}
			return err
		}
		worktreeName = arg

		info = &worktree.WorktreeInfo{
			Type:         worktree.Local,
			Repo:         repoName,
			WorktreeName: arg,
		}
	}

	baseDir := config.GetWorktreeBase()

	var repoPath string
	if info.Type == worktree.Local {
		gitCommonDir, err := git.GetGitCommonDir(worktreePath)
		if err != nil {
			return fmt.Errorf("failed to get git directory: %w", err)
		}
		repoPath = filepath.Dir(gitCommonDir)
	} else {
		repoPath = info.GetRepoPath(baseDir)
	}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		fmt.Printf("Worktree '%s' not found, nothing to remove\n", worktreeName)
		return nil
	}

	// Get branch name
	branch, err := git.GetCurrentBranch(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to get branch name: %w", err)
	}

	// Check for uncommitted changes if not forced
	if !forceFlag && git.HasUncommittedChanges(worktreePath) {
		p := prompter.New(os.Stdin, os.Stdout, os.Stderr)
		confirm, err := p.Confirm(fmt.Sprintf("Worktree '%s' has uncommitted changes. Remove anyway?", worktreeName), false)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if !confirm {
			fmt.Println("Cancelled - no changes were made")
			return nil
		}
		// Force remove since user confirmed
		forceFlag = true
	}

	// Remove worktree
	if err := worktree.Remove(repoPath, worktreePath, branch, forceFlag); err != nil {
		return err
	}

	fmt.Printf("Removed worktree: %s\n", worktreeName)
	return nil
}

func findWorktree(baseDir, name string) (repoName, worktreePath string, err error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", "", fmt.Errorf("worktree base directory not found: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		possiblePath := filepath.Join(baseDir, entry.Name(), name)
		if _, err := os.Stat(possiblePath); err == nil {
			return entry.Name(), possiblePath, nil
		}
	}

	return "", "", fmt.Errorf("worktree '%s' not found", name)
}

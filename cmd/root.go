package cmd

import (
	"os"
	"strings"

	"github.com/ffalor/gh-worktree/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Used for flags
	forceFlag bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gh-worktree [url|name]",
	Short: "Create and manage git worktrees from GitHub PRs and Issues",
	Long: `gh-worktree is a GitHub CLI extension that helps you create git worktrees
from GitHub pull requests, issues, or local branch names.

Examples:
  # Create worktree from PR URL
  gh worktree https://github.com/owner/repo/pull/123

  # Create worktree from Issue URL
  gh worktree https://github.com/owner/repo/issues/456

  # Create local worktree (from within a repo)
  gh worktree my-feature-branch

  # List all worktrees
  gh worktree list

  # Remove a worktree
  gh worktree remove pr_123`,
	Args: cobra.ArbitraryArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.Load()
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// If arguments provided, treat as create command
		if len(args) > 0 {
			return createCmd.RunE(cmd, args)
		}
		// Show help if no args
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	// Parse arguments to handle the case where a URL or branch name is passed
	// without the 'create' subcommand
	if len(os.Args) > 1 {
		// Find the first non-flag argument
		firstNonFlagIdx := -1
		for i, arg := range os.Args[1:] {
			if !strings.HasPrefix(arg, "-") {
				firstNonFlagIdx = i + 1
				break
			}
		}

		if firstNonFlagIdx > 0 {
			firstNonFlag := os.Args[firstNonFlagIdx]
			// If it doesn't look like a known subcommand, insert "create"
			if !isKnownCommand(firstNonFlag) {
				// Insert "create" before the first non-flag argument
				newArgs := make([]string, 0, len(os.Args)+1)
				newArgs = append(newArgs, os.Args[0])
				newArgs = append(newArgs, os.Args[1:firstNonFlagIdx]...) // flags
				newArgs = append(newArgs, "create")
				newArgs = append(newArgs, os.Args[firstNonFlagIdx:]...) // rest
				os.Args = newArgs
			}
		}
	}

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// isKnownCommand checks if the argument is a known subcommand
func isKnownCommand(arg string) bool {
	knownCommands := []string{"create", "list", "remove", "help", "completion"}
	for _, cmd := range knownCommands {
		if arg == cmd {
			return true
		}
	}
	return false
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&forceFlag, "force", "f", false, "force operation without prompts")
}

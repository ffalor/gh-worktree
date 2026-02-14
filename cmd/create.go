package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	gh "github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/prompter"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/ffalor/gh-wt/internal/action"
	"github.com/ffalor/gh-wt/internal/config"
	"github.com/ffalor/gh-wt/internal/execext"
	"github.com/ffalor/gh-wt/internal/git"
	"github.com/ffalor/gh-wt/internal/logger"
	"github.com/ffalor/gh-wt/internal/worktree"
	"github.com/spf13/cobra"
)

// addCmd represents the add command.
var addCmd = &cobra.Command{
	Use:   "add [url|name]",
	Short: "Add a new worktree",
	Long: `Add a new git worktree from either:
 - A GitHub pull request URL or number
 - A GitHub issue URL or number
 - A name to use for the new worktree and branch
`,
	Aliases: []string{"create"},
	Args:    cobra.RangeArgs(0, 1),
	RunE:    runAdd,
}

func init() {
	addCmd.Flags().BoolVarP(&useExistingFlag, "use-existing", "e", false, "use existing branch if it exists")
	addCmd.Flags().StringVar(&prFlag, "pr", "", "PR number, PR URL, or git remote URL with PR ref")
	addCmd.Flags().StringVar(&issueFlag, "issue", "", "issue number, issue URL, or git remote URL with issue ref")
	addCmd.Flags().StringVar(&actionFlag, "action", "", "action to run after worktree creation")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Determine the type of input
	if prFlag != "" {
		return createFromPR(prFlag)
	}
	if issueFlag != "" {
		return createFromIssue(issueFlag)
	}
	if len(args) == 0 {
		return cmd.Help()
	}

	// This is the main entry point for creating a worktree
	arg := args[0]
	worktreeType, err := DetermineWorktreeType(arg)
	if err != nil {
		return err
	}

	switch worktreeType {
	case worktree.PR:
		return createFromPR(arg)
	case worktree.Issue:
		return createFromIssue(arg)
	default:
		return createFromLocal(arg)
	}
}

// createFromPR handles creation from a PR URL or number.
func createFromPR(value string) error {
	Log.Infof("Fetching Pull Request info...\n")
	args := []string{"pr", "view", value, "--json", "number,title,headRefName,url"}
	stdout, stderr, err := gh.Exec(args...)
	if err != nil {
		return fmt.Errorf("failed to fetch PR info: %s\n%s", err, stderr.String())
	}

	var prInfo struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		HeadRefName string `json:"headRefName"`
		URL         string `json:"url"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &prInfo); err != nil {
		return fmt.Errorf("failed to parse PR info: %w", err)
	}

	repo, err := repository.Current()
	if err != nil {
		return err
	}

	info := &worktree.WorktreeInfo{
		Type:         worktree.PR,
		Owner:        repo.Owner,
		Repo:         repo.Name,
		Number:       prInfo.Number,
		BranchName:   prInfo.HeadRefName,
		WorktreeName: fmt.Sprintf("pr_%d", prInfo.Number),
	}

	Log.Outf(logger.Green, "Creating worktree for PR #%d: %s\n", info.Number, prInfo.Title)

	// Fetch the PR ref
	prRef := fmt.Sprintf("refs/pull/%d/head", info.Number)
	Log.Infof("Fetching PR #%d...\n", info.Number)
	if err := git.Fetch(prRef); err != nil {
		return fmt.Errorf("failed to fetch PR: %w", err)
	}

	return createWorktree(info, "FETCH_HEAD")
}

// createFromIssue handles creation from an Issue URL or number.
func createFromIssue(value string) error {
	Log.Infof("Fetching Issue info...\n")
	args := []string{"issue", "view", value, "--json", "number,title,url"}
	stdout, stderr, err := gh.Exec(args...)
	if err != nil {
		return fmt.Errorf("failed to fetch Issue info: %s\n%s", err, stderr.String())
	}

	var issueInfo struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &issueInfo); err != nil {
		return fmt.Errorf("failed to parse issue info: %w", err)
	}

	repo, err := repository.Current()
	if err != nil {
		return err
	}

	branchName := fmt.Sprintf("issue_%d", issueInfo.Number)
	info := &worktree.WorktreeInfo{
		Type:         worktree.Issue,
		Owner:        repo.Owner,
		Repo:         repo.Name,
		Number:       issueInfo.Number,
		BranchName:   branchName,
		WorktreeName: branchName,
	}

	Log.Outf(logger.Green, "Creating worktree for Issue #%d: %s\n", info.Number, issueInfo.Title)
	return createWorktree(info, "HEAD") // Issues start from HEAD
}

// createFromLocal handles creation from a local branch name.
func createFromLocal(name string) error {
	if !git.IsGitRepository(".") {
		return fmt.Errorf("not in a git repository")
	}

	// Get repo name using the shared helper
	repoName, err := git.GetRepoName()
	if err != nil {
		return err
	}

	// Sanitize the name for the branch
	sanitizedBranchName := SanitizeBranchName(name)

	info := &worktree.WorktreeInfo{
		Type:         worktree.Local,
		Repo:         repoName,
		BranchName:   sanitizedBranchName,
		WorktreeName: name, // Worktree directory keeps the original name
	}

	return createWorktree(info, "HEAD")
}

// createWorktree is the central function that performs the creation.
// It contains all the logic for path generation, user prompts, and calling the worktree package.
func createWorktree(info *worktree.WorktreeInfo, startPoint string) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}
	baseDir := cfg.WorktreeBase
	worktreePath := filepath.Join(baseDir, info.Repo, info.WorktreeName)
	absPath, _ := filepath.Abs(worktreePath)

	// Check conditions
	branchExists := git.BranchExists(info.BranchName)
	worktreeDirExists := worktree.Exists(worktreePath)
	worktreeGitRegistered := git.WorktreeIsRegistered(worktreePath)

	// Build the prompt message if there are conflicts
	hasConflict := worktreeDirExists || worktreeGitRegistered || branchExists

	if hasConflict {
		p := prompter.New(os.Stdin, os.Stdout, os.Stderr)

		// Build the "This will:" message
		var message strings.Builder
		message.WriteString("Target: create worktree for '")
		message.WriteString(info.BranchName)
		message.WriteString("'\n\n")
		message.WriteString("This will:\n")

		// Determine what worktree info we can get
		currentBranch := ""
		if worktreeGitRegistered {
			currentBranch, _ = git.GetWorktreeBranch(worktreePath)
		}

		// Add worktree actions
		if worktreeDirExists && worktreeGitRegistered {
			// Valid worktree
			if currentBranch != "" {
				message.WriteString("- Remove worktree at ")
				message.WriteString(absPath)
				message.WriteString(" (currently on branch '")
				message.WriteString(currentBranch)
				message.WriteString("')\n")
			} else {
				message.WriteString("- Remove worktree at ")
				message.WriteString(absPath)
				message.WriteString("\n")
			}
		} else if worktreeGitRegistered {
			// Invalid worktree (git only)
			message.WriteString("- Remove stale worktree record at ")
			message.WriteString(absPath)
			message.WriteString("\n")
		} else if worktreeDirExists { // Disk only - just remove directory
			message.WriteString("- Remove directory at ")
			message.WriteString(absPath)
			message.WriteString("\n")
		}

		// Add branch actions
		if branchExists {
			message.WriteString("- Delete existing branch '")
			message.WriteString(info.BranchName)
			message.WriteString("'\n")
		}

		// Add create action
		message.WriteString("- Create worktree and branch for '")
		message.WriteString(info.BranchName)
		message.WriteString("'\n")

		// Check worktree for uncommitted changes
		if worktreeDirExists && git.IsGitRepository(worktreePath) {
			if git.HasUncommittedChanges(worktreePath) {
				message.WriteString("\n⚠️  WARNING: Worktree at ")
				message.WriteString(absPath)
				message.WriteString(" has uncommitted changes that will be PERMANENTLY DELETED. Consider committing or stashing changes first.\n")
			}
		}

		// Check branch for uncommitted changes (only if branch exists and has a worktree)
		if branchExists {
			// Find the worktree for this branch
			worktrees, err := git.GetWorktreeInfo()
			if err == nil {
				for _, wt := range worktrees {
					if wt.Branch == info.BranchName {
						if git.HasUncommittedChanges(wt.Path) {
							message.WriteString("\n⚠️ WARNING: Branch '")
							message.WriteString(info.BranchName)
							message.WriteString("' has uncommitted changes that will be PERMANENTLY DELETED. Consider committing or stashing changes first.\n")
						}
						break
					}
				}
			}
		}

		message.WriteString("\nOverwrite?")

		// If force flag is set, skip the prompt
		if !forceFlag {
			overwrite, err := p.Confirm(message.String(), false)
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}
			if !overwrite {
				Log.Warnf("Cancelled - no changes made\n")
				return nil
			}
		}

		// Perform cleanup based on what exists
		if worktreeDirExists && worktreeGitRegistered {
			// Valid worktree - use git to remove
			if err := git.WorktreeRemove(worktreePath, true); err != nil {
				return fmt.Errorf("failed to remove worktree: %w", err)
			}
		} else if worktreeDirExists {
			// Disk only - just remove directory
			if err := os.RemoveAll(worktreePath); err != nil {
				return fmt.Errorf("failed to remove directory: %w", err)
			}
		} else if worktreeGitRegistered {
			// Git only - prune the record
			if err := git.WorktreePrune(); err != nil {
				return fmt.Errorf("failed to prune worktree: %w", err)
			}
		}

		// Delete branch if it exists
		if branchExists {
			Log.Infof("Deleting existing branch '%s'...\n", info.BranchName)
			if err := git.BranchDelete(info.BranchName, true); err != nil {
				return fmt.Errorf("failed to delete branch: %w", err)
			}
		}
	}

	// Create the new worktree.
	err = worktree.Create(worktreePath, info.BranchName, startPoint)
	if err != nil {
		// Simple cleanup: if creation fails, try to remove the directory if it was created.
		if worktree.Exists(worktreePath) {
			os.RemoveAll(worktreePath)
		}
		return err
	}

	printSuccess(absPath)

	if actionFlag != "" {
		if err := action.Execute(context.Background(), &action.ExecuteOptions{
			ActionName:   actionFlag,
			WorktreePath: absPath,
			Info:         info,
			CLIArgs:      cliArgs,
			Logger:       Log,
			Stdin:        os.Stdin,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
			Env:          os.Environ(),
		}); err != nil {
			// Don't fail the whole operation if the action fails, just print a warning
			Log.Warnf("\n⚠️  Action '%s' failed: %v\n", actionFlag, err)
		}
	} else if cliArgs != "" {
		// Run CLI args directly in the worktree if no action is specified
		Log.Outf(logger.Magenta, "\nRunning in worktree: %s\n", cliArgs)

		if err := execext.RunCommand(context.Background(), &execext.RunCommandOptions{
			Command: cliArgs,
			Dir:     absPath,
			Env:     os.Environ(),
			Stdin:   os.Stdin,
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
		}); err != nil {
			Log.Warnf("\n⚠️  Command '%s' failed: %v\n", cliArgs, err)
		}
	}

	return nil
}

// printSuccess prints the final success message.
func printSuccess(path string) {
	Log.Outf(logger.Green, "\nWorktree created successfully!\n")
	Log.Outf(logger.Default, "Location: %s\n", path)
	Log.Outf(logger.Default, "\nTo switch to the worktree:\n")
	Log.Outf(logger.Cyan, "  cd %s\n", path)
}

// SanitizeBranchName is moved from types.go.
func SanitizeBranchName(name string) string {
	invalidChars := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return invalidChars.ReplaceAllString(name, "_")
}

// DetermineWorktreeType determines the type of worktree based on the input
// Returns the worktree type and an error message if invalid.
func DetermineWorktreeType(input string) (worktree.WorktreeType, error) {
	u, err := url.Parse(input)
	if err != nil {
		return worktree.Local, nil
	}

	if u.Scheme == "" {
		return worktree.Local, nil
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return worktree.Local, nil
	}

	prPattern := regexp.MustCompile(`^/[^/]+/[^/]+/pull/\d+(?:/.*)?$`)
	if prPattern.MatchString(u.Path) {
		return worktree.PR, nil
	}

	issuePattern := regexp.MustCompile(`^/[^/]+/[^/]+/issues/\d+(?:/.*)?$`)
	if issuePattern.MatchString(u.Path) {
		return worktree.Issue, nil
	}

	return worktree.Local, nil
}

var (
	useExistingFlag bool
	prFlag          string
	issueFlag       string
	actionFlag      string
)

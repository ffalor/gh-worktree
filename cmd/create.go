package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	gh "github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/prompter"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/ffalor/gh-worktree/internal/config"
	"github.com/ffalor/gh-worktree/internal/git"
	"github.com/ffalor/gh-worktree/internal/worktree"
	"github.com/spf13/cobra"
)

// WorktreeType represents the type of worktree
type WorktreeType string

const (
	Issue WorktreeType = "issue"
	PR    WorktreeType = "pr"
	Local WorktreeType = "local"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create [url|name]",
	Short: "Create a new worktree from a GitHub URL or branch name",
	Long: `Create a new git worktree from either:
- A GitHub pull request URL or number
- A GitHub issue URL or number
- A local branch name (when run from within a git repository)`,
	Args: cobra.RangeArgs(0, 1),
	RunE: runCreate,
}

var (
	useExistingFlag bool
	prFlag          string
	issueFlag       string
)

// WorktreeInfo is a new struct, moved from the worktree package.
// It acts as a data container for the create command.
type WorktreeInfo struct {
	Type         WorktreeType
	Owner        string
	Repo         string
	Number       int
	BranchName   string
	WorktreeName string
}

func init() {
	createCmd.Flags().BoolVarP(&useExistingFlag, "use-existing", "e", false, "use existing branch if it exists")
	createCmd.Flags().StringVar(&prFlag, "pr", "", "PR number, PR URL, or git remote URL with PR ref")
	createCmd.Flags().StringVar(&issueFlag, "issue", "", "issue number, issue URL, or git remote URL with issue ref")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
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
	case PR:
		return createFromPR(arg)
	case Issue:
		return createFromIssue(arg)
	default:
		return createFromLocal(arg)
	}
}

// createFromPR handles creation from a PR URL or number.
func createFromPR(value string) error {
	fmt.Println("Fetching Pull Request info...")
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

	info := &WorktreeInfo{
		Type:         PR,
		Owner:        repo.Owner,
		Repo:         repo.Name,
		Number:       prInfo.Number,
		BranchName:   prInfo.HeadRefName,
		WorktreeName: fmt.Sprintf("pr_%d", prInfo.Number),
	}

	fmt.Printf("Creating worktree for PR #%d: %s\n", info.Number, prInfo.Title)

	// Fetch the PR ref
	prRef := fmt.Sprintf("refs/pull/%d/head", info.Number)
	fmt.Printf("Fetching PR #%d...\n", info.Number)
	if err := git.Fetch(prRef); err != nil {
		return fmt.Errorf("failed to fetch PR: %w", err)
	}

	return createWorktree(info, "FETCH_HEAD")
}

// createFromIssue handles creation from an Issue URL or number.
func createFromIssue(value string) error {
	fmt.Println("Fetching Issue info...")
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
	info := &WorktreeInfo{
		Type:         Issue,
		Owner:        repo.Owner,
		Repo:         repo.Name,
		Number:       issueInfo.Number,
		BranchName:   branchName,
		WorktreeName: branchName,
	}

	fmt.Printf("Creating worktree for Issue #%d: %s\n", info.Number, issueInfo.Title)
	return createWorktree(info, "HEAD") // Issues start from HEAD
}

// createFromLocal handles creation from a local branch name.
func createFromLocal(name string) error {
	if !git.IsGitRepository(".") {
		return fmt.Errorf("not in a git repository")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Sanitize the name for the branch
	sanitizedBranchName := SanitizeBranchName(name)

	info := &WorktreeInfo{
		Type:         Local,
		Repo:         filepath.Base(cwd),
		BranchName:   sanitizedBranchName,
		WorktreeName: name, // Worktree directory keeps the original name
	}

	return createWorktree(info, "HEAD")
}

// createWorktree is the central function that performs the creation.
// It contains all the logic for path generation, user prompts, and calling the worktree package.
func createWorktree(info *WorktreeInfo, startPoint string) error {
	baseDir := config.GetWorktreeBase()
	worktreePath := filepath.Join(baseDir, info.Repo, info.WorktreeName)
	absPath, _ := filepath.Abs(worktreePath)

	// 1. Check if the worktree directory already exists.
	if worktree.Exists(worktreePath) {
		return fmt.Errorf("worktree directory already exists: %s", absPath)
	}

	// 2. Check if the branch exists and handle it.
	if git.BranchExists(info.BranchName) {
		if useExistingFlag {
			fmt.Printf("Attaching to existing branch '%s'...\n", info.BranchName)
			if err := worktree.Attach(worktreePath, info.BranchName); err != nil {
				return err
			}
			printSuccess(absPath)
			return nil
		}

		// Prompt the user for action
		p := prompter.New(os.Stdin, os.Stdout, os.Stderr)
		options := []string{"Overwrite (delete and recreate)", "Attach (use existing branch)", "Cancel"}
		choice, err := p.Select(fmt.Sprintf("Branch '%s' already exists. What would you like to do?", info.BranchName), "", options)
		if err != nil {
			return errors.New("operation cancelled")
		}

		switch choice {
		case 0: // Overwrite
			fmt.Printf("Deleting existing branch '%s'...\n", info.BranchName)
			if err := git.BranchDelete(info.BranchName, true); err != nil {
				return fmt.Errorf("failed to delete branch: %w", err)
			}
			// Continue to creation
		case 1: // Attach
			fmt.Printf("Attaching to existing branch '%s'...\n", info.BranchName)
			if err := worktree.Attach(worktreePath, info.BranchName); err != nil {
				return err
			}
			printSuccess(absPath)
			return nil
		case 2: // Cancel
			return errors.New("operation cancelled")
		}
	}

	// 3. Create the new worktree.
	fmt.Printf("Creating branch '%s'...\n", info.BranchName)
	err := worktree.Create(worktreePath, info.BranchName, startPoint)
	if err != nil {
		// Simple cleanup: if creation fails, try to remove the directory if it was created.
		if worktree.Exists(worktreePath) {
			os.RemoveAll(worktreePath)
		}
		return err
	}

	printSuccess(absPath)
	return nil
}

// printSuccess prints the final success message.
func printSuccess(path string) {
	fmt.Printf("\nWorktree created successfully!\n")
	fmt.Printf("Location: %s\n", path)
	fmt.Printf("\nTo switch to the worktree:\n")
	fmt.Printf("  cd %s\n", path)
}

// SanitizeBranchName is moved from types.go
func SanitizeBranchName(name string) string {
	invalidChars := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return invalidChars.ReplaceAllString(name, "_")
}

// DetermineWorktreeType determines the type of worktree based on the input
// Returns the worktree type and an error message if invalid
func DetermineWorktreeType(input string) (WorktreeType, error) {
	u, err := url.Parse(input)
	if err != nil {
		return Local, nil
	}

	if u.Scheme == "" {
		return Local, nil
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return Local, nil
	}

	var prPattern = regexp.MustCompile(`^/[^/]+/[^/]+/pull/\d+(?:/.*)?$`)
	if prPattern.MatchString(u.Path) {
		return PR, nil
	}

	var issuePattern = regexp.MustCompile(`^/[^/]+/[^/]+/issues/\d+(?:/.*)?$`)
	if issuePattern.MatchString(u.Path) {
		return Issue, nil
	}

	return Local, nil
}

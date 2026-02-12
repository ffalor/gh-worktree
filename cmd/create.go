package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gh "github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/prompter"
	"github.com/cli/go-gh/v2/pkg/repository"
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

func init() {
	createCmd.Flags().BoolVarP(&useExistingFlag, "use-existing", "e", false, "use existing branch if it exists")
	createCmd.Flags().StringVar(&prFlag, "pr", "", "PR number, PR URL, or git remote URL with PR ref")
	createCmd.Flags().StringVar(&issueFlag, "issue", "", "issue number, issue URL, or git remote URL with issue ref")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	if prFlag != "" {
		return handlePRFlag(prFlag)
	}

	if issueFlag != "" {
		return handleIssueFlag(issueFlag)
	}

	if len(args) == 0 {
		return cmd.Help()
	}

	arg := args[0]
	worktreeType, err := worktree.DetermineWorktreeType(arg)
	if err != nil {
		return err
	}

	switch worktreeType {
	case worktree.PR:
		return handlePRFlag(arg)
	case worktree.Issue:
		return handleIssueFlag(arg)
	default:
		return handleLocalArgument(arg)
	}
}

func handlePRFlag(value string) error {
	args := []string{"pr", "view", value, "--json", "number,title,headRefName,url,headRepositoryOwner,headRepository"}
	stdout, stderr, err := gh.Exec(args...)

	if err != nil {
		return fmt.Errorf("command: gh %s\nerror: %s", strings.Join(args, " "), stderr.String())
	}

	var prInfo struct {
		Number         int    `json:"number"`
		Title          string `json:"title"`
		HeadRefName    string `json:"headRefName"`
		URL            string `json:"url"`
		HeadRepository struct {
			Name string `json:"name"`
		} `json:"headRepository"`
		HeadRepositoryOwner struct {
			Login string `json:"login"`
		} `json:"headRepositoryOwner"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &prInfo); err != nil {
		return fmt.Errorf("failed to parse PR info: %w", err)
	}

	fmt.Printf("Creating worktree for PR #%d: %s\n", prInfo.Number, prInfo.Title)
	fmt.Printf("Checking out branch: %s\n", prInfo.HeadRefName)

	owner := prInfo.HeadRepositoryOwner.Login
	repo := prInfo.HeadRepository.Name
	info := &worktree.WorktreeInfo{
		Type:         worktree.PR,
		Owner:        owner,
		Repo:         repo,
		Number:       prInfo.Number,
		BranchName:   prInfo.HeadRefName,
		WorktreeName: fmt.Sprintf("pr_%d", prInfo.Number),
		CloneURL:     fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
	}

	return createWorktree(info)
}

func handleIssueFlag(value string) error {
	args := []string{"issue", "view", value, "--json", "number,title,url"}
	stdout, stderr, err := gh.Exec(args...)

	if err != nil {
		return fmt.Errorf("command: gh %s\nerror: %s", strings.Join(args, " "), stderr.String())
	}

	var issueInfo struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &issueInfo); err != nil {
		return fmt.Errorf("failed to parse issue info: %w", err)
	}

	fmt.Printf("Creating worktree for issue #%d: %s\n", issueInfo.Number, issueInfo.Title)

	repoURL := strings.Split(issueInfo.URL, "/issues/")[0]
	repo, err := repository.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("failed to parse repository from URL: %w", err)
	}

	info := &worktree.WorktreeInfo{
		Type:         worktree.Issue,
		Owner:        repo.Owner,
		Repo:         repo.Name,
		Number:       issueInfo.Number,
		BranchName:   fmt.Sprintf("issue_%d", issueInfo.Number),
		WorktreeName: fmt.Sprintf("issue_%d", issueInfo.Number),
		CloneURL:     fmt.Sprintf("https://github.com/%s/%s.git", repo.Owner, repo.Name),
	}

	return createWorktree(info)
}

func handleLocalArgument(name string) error {
	if !git.IsGitRepository(".") {
		return fmt.Errorf("not in a git repository: cannot create local worktree from branch name '%s'", name)
	}

	info, err := worktree.ParseArgument(name)
	if err != nil {
		return err
	}

	return createWorktree(info)
}

func createWorktree(info *worktree.WorktreeInfo) error {
	baseDir := config.GetWorktreeBase()
	worktreePath := info.GetWorktreePath(baseDir)
	repoPath := info.GetRepoPath(baseDir)

	if worktree.WorktreeExists(worktreePath) {
		if forceFlag {
			fmt.Printf("Force flag set. Removing existing worktree...\n")
			branch, _ := git.GetCurrentBranch(worktreePath)
			if err := worktree.Remove(repoPath, worktreePath, branch, true); err != nil {
				return fmt.Errorf("failed to remove existing worktree: %w", err)
			}
		} else {
			p := prompter.New(os.Stdin, os.Stdout, os.Stderr)
			confirm, err := p.Confirm(fmt.Sprintf("Worktree already exists at %s. Overwrite?", worktreePath), false)
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			if !confirm {
				fmt.Println("Operation cancelled")
				return nil
			}
			branch, _ := git.GetCurrentBranch(worktreePath)
			if err := worktree.Remove(repoPath, worktreePath, branch, true); err != nil {
				return fmt.Errorf("failed to remove existing worktree: %w", err)
			}
		}
	}

	creator := worktree.NewCreatorWithCheck(func(branchName string) worktree.BranchAction {
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

	err := creator.Create(info)
	if err != nil {
		creator.Cleanup()
		return err
	}

	return nil
}

func branchExists(repoPath, branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoPath
	err := cmd.Run()
	return err == nil
}

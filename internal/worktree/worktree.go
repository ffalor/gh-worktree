package worktree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/ffalor/gh-worktree/internal/config"
	"github.com/ffalor/gh-worktree/internal/git"
)

var ErrCancelled = errors.New("cancelled")

type BranchAction int

const (
	BranchActionOverwrite BranchAction = iota
	BranchActionAttach
	BranchActionCancel
)

// Creator handles worktree creation with cleanup support
type Creator struct {
	baseDir         string
	createdDirs     []string
	createdBranches []string
	repoPath        string
	branchCheck     func(string) BranchAction
}

// NewCreator creates a new worktree creator
func NewCreator() *Creator {
	return &Creator{
		baseDir: config.GetWorktreeBase(),
	}
}

// NewCreatorWithCheck creates a new worktree creator with a branch check callback
func NewCreatorWithCheck(check func(string) BranchAction) *Creator {
	return &Creator{
		baseDir:     config.GetWorktreeBase(),
		branchCheck: check,
	}
}

// Create creates a new worktree from the given info
func (c *Creator) Create(info *WorktreeInfo) error {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Fetch details based on type
	switch info.Type {
	case Issue:
		if err := c.fetchIssueDetails(client, info); err != nil {
			return err
		}
	case PR:
		if err := c.fetchPRDetails(client, info); err != nil {
			return err
		}
	case Local:
		// No API call needed
	}

	return c.setupWorktree(info)
}

// Cleanup removes all created resources (for rollback on error)
func (c *Creator) Cleanup() {
	// Remove created directories
	for _, dir := range c.createdDirs {
		_ = os.RemoveAll(dir)
	}
	// Remove created branches
	for _, branch := range c.createdBranches {
		_ = git.BranchDelete(c.repoPath, branch, true)
	}
}

func (c *Creator) fetchIssueDetails(client *api.RESTClient, info *WorktreeInfo) error {
	response := struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}{}

	path := fmt.Sprintf("repos/%s/%s/issues/%d", info.Owner, info.Repo, info.Number)
	if err := client.Get(path, &response); err != nil {
		return fmt.Errorf("failed to fetch issue: %w", err)
	}

	info.BranchName = fmt.Sprintf("issue_%d", response.Number)
	fmt.Printf("Creating worktree for issue #%d: %s\n", response.Number, response.Title)
	return nil
}

func (c *Creator) fetchPRDetails(client *api.RESTClient, info *WorktreeInfo) error {
	response := struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
	}{}

	path := fmt.Sprintf("repos/%s/%s/pulls/%d", info.Owner, info.Repo, info.Number)
	if err := client.Get(path, &response); err != nil {
		return fmt.Errorf("failed to fetch PR: %w", err)
	}

	info.BranchName = response.Head.Ref
	fmt.Printf("Creating worktree for PR #%d: %s\n", response.Number, response.Title)
	fmt.Printf("Checking out branch: %s\n", info.BranchName)
	return nil
}

func (c *Creator) setupWorktree(info *WorktreeInfo) error {
	worktreeBase := filepath.Join(c.baseDir, info.Repo)
	worktreePath := filepath.Join(worktreeBase, info.WorktreeName)
	c.repoPath = filepath.Join(worktreeBase, BareDir)

	// Create worktree base directory
	if err := os.MkdirAll(worktreeBase, 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Clone bare repo if it doesn't exist
	if _, err := os.Stat(c.repoPath); os.IsNotExist(err) {
		fmt.Printf("Cloning %s/%s...\n", info.Owner, info.Repo)
		repoSpec := fmt.Sprintf("%s/%s", info.Owner, info.Repo)
		if err := git.CloneBare(worktreeBase, repoSpec, BareDir); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
		if err := git.ConfigRemote(c.repoPath); err != nil {
			return fmt.Errorf("failed to configure remote: %w", err)
		}
	}

	// Check if worktree exists on disk
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		return fmt.Errorf("worktree already exists: %s", worktreePath)
	}

	// Check if git still has a record of this worktree (even though it doesn't exist on disk)
	// and remove it if necessary
	_ = git.WorktreeRemove(c.repoPath, worktreePath)

	// Determine the branch name we'll use
	var branchName string
	switch info.Type {
	case Issue:
		branchName = fmt.Sprintf("issue_%d", info.Number)
	case PR:
		branchName = info.BranchName
		if branchName == "" {
			branchName = fmt.Sprintf("pr_%d", info.Number)
		}
	case Local:
		branchName = info.BranchName
	}

	// Call the branch check callback if provided
	if c.branchCheck != nil {
		action := c.branchCheck(branchName)
		switch action {
		case BranchActionCancel:
			return ErrCancelled
		case BranchActionAttach:
			// Skip branch creation, use existing branch
			switch info.Type {
			case Issue, Local:
				fmt.Printf("Attaching to existing branch '%s'...\n", branchName)
				if err := git.WorktreeAddFromBranch(c.repoPath, branchName, worktreePath); err != nil {
					return fmt.Errorf("failed to attach to worktree: %w", err)
				}
			case PR:
				prRef := fmt.Sprintf("refs/pull/%d/head", info.Number)
				fmt.Printf("Fetching PR #%d...\n", info.Number)
				if err := git.Fetch(c.repoPath, prRef); err != nil {
					return fmt.Errorf("failed to fetch PR: %w", err)
				}
				fmt.Printf("Attaching to existing branch '%s'...\n", branchName)
				if err := git.WorktreeAddFromBranch(c.repoPath, branchName, worktreePath); err != nil {
					return fmt.Errorf("failed to attach to worktree: %w", err)
				}
			}
			fmt.Printf("\nWorktree created successfully!\n")
			fmt.Printf("Location: %s\n", worktreePath)
			absPath, _ := filepath.Abs(worktreePath)
			fmt.Printf("\nTo switch to the worktree:\n")
			fmt.Printf("  cd %s\n", absPath)
			return nil
		}
		// BranchActionOverwrite: continue to delete and recreate
	}

	// Track for potential cleanup
	c.createdDirs = append(c.createdDirs, worktreePath)

	switch info.Type {
	case Issue, Local:
		fmt.Printf("Creating branch '%s'...\n", branchName)
		if err := git.WorktreeAdd(c.repoPath, branchName, worktreePath); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
		c.createdBranches = append(c.createdBranches, branchName)

	case PR:
		prRef := fmt.Sprintf("refs/pull/%d/head", info.Number)
		fmt.Printf("Fetching PR #%d...\n", info.Number)
		if err := git.Fetch(c.repoPath, prRef); err != nil {
			return fmt.Errorf("failed to fetch PR: %w", err)
		}

		fmt.Printf("Creating worktree for branch '%s'...\n", branchName)
		if err := git.WorktreeAddFromRef(c.repoPath, branchName, worktreePath, "FETCH_HEAD"); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
		c.createdBranches = append(c.createdBranches, branchName)
	}

	fmt.Printf("\nWorktree created successfully!\n")
	fmt.Printf("Location: %s\n", worktreePath)

	absPath, _ := filepath.Abs(worktreePath)
	fmt.Printf("\nTo switch to the worktree:\n")
	fmt.Printf("  cd %s\n", absPath)

	return nil
}

// Remove removes a worktree and its branch
func Remove(repoPath, worktreePath, branch string, force bool) error {
	// Check for uncommitted changes if not forced
	if !force && git.HasUncommittedChanges(worktreePath) {
		return fmt.Errorf("worktree has uncommitted changes")
	}

	// Remove worktree
	if err := git.WorktreeRemove(repoPath, worktreePath); err != nil {
		// Try manual removal if git worktree remove fails
		_ = os.RemoveAll(worktreePath)
	}

	// Delete branch
	if err := git.BranchDelete(repoPath, branch, true); err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	return nil
}

// List returns all worktrees for a repository
func List(repoPath string) ([]WorktreeListItem, error) {
	worktreePaths, err := git.ListWorktrees(repoPath)
	if err != nil {
		return nil, err
	}

	var items []WorktreeListItem
	for _, path := range worktreePaths {
		// Skip the bare repo
		if filepath.Base(path) == BareDir {
			continue
		}

		item := WorktreeListItem{
			Path: path,
			Name: filepath.Base(path),
		}

		// Get branch name
		if branch, err := git.GetCurrentBranch(path); err == nil {
			item.Branch = branch
		}

		// Check for uncommitted changes
		item.HasChanges = git.HasUncommittedChanges(path)

		// Get modification time
		if info, err := os.Stat(path); err == nil {
			item.LastModTime = info.ModTime().Unix()
		}

		items = append(items, item)
	}

	return items, nil
}

// WorktreeExists checks if a worktree already exists
func WorktreeExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

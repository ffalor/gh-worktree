package worktree

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ffalor/gh-worktree/internal/git"
)

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

// WorktreeType represents the type of worktree
type WorktreeType string

const (
	Issue WorktreeType = "issue"
	PR    WorktreeType = "pr"
	Local WorktreeType = "local"
)

// WorktreeInfo holds information about a worktree to create
type WorktreeInfo struct {
	Type         WorktreeType
	Owner        string
	Repo         string
	Number       int
	BranchName   string
	WorktreeName string
}

// ParseArgument parses the command line argument
func ParseArgument(arg string) (*WorktreeInfo, error) {
	if isGitHubURL(arg) {
		return parseGitHubURL(arg)
	}

	return parseLocalName(arg)
}

// isGitHubURL checks if the string is a GitHub URL
func isGitHubURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "https" && strings.Contains(u.Host, "github.com")
}

// parseGitHubURL parses a GitHub URL (PR or Issue)
func parseGitHubURL(githubURL string) (*WorktreeInfo, error) {
	u, err := url.Parse(githubURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid GitHub URL format")
	}

	owner := parts[0]
	repo := parts[1]
	itemType := parts[2]
	numberStr := parts[3]

	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return nil, fmt.Errorf("invalid issue/PR number: %w", err)
	}

	info := &WorktreeInfo{
		Owner:  owner,
		Repo:   repo,
		Number: number,
	}

	switch itemType {
	case "issues":
		info.Type = Issue
		info.WorktreeName = fmt.Sprintf("issue_%d", number)
	case "pull":
		info.Type = PR
		info.WorktreeName = fmt.Sprintf("pr_%d", number)
	default:
		return nil, fmt.Errorf("unsupported URL type: %s (expected 'issues' or 'pull')", itemType)
	}

	return info, nil
}

// parseLocalName parses a local name argument
func parseLocalName(name string) (*WorktreeInfo, error) {
	gitDir, err := git.GetGitDir(".")
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}

	absGitDir, err := filepath.Abs(gitDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	var repoPath, repoName string
	if git.IsBareRepository(".") {
		repoPath = absGitDir
		repoName = filepath.Base(filepath.Dir(repoPath))
	} else {
		repoPath = filepath.Dir(absGitDir)
		repoName = filepath.Base(repoPath)
	}

	validBranchName := SanitizeBranchName(name)

	return &WorktreeInfo{
		Type:         Local,
		BranchName:   validBranchName,
		WorktreeName: name,
		Repo:         repoName,
	}, nil
}

// SanitizeBranchName sanitizes a string for use as a git branch name
func SanitizeBranchName(name string) string {
	invalidChars := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return invalidChars.ReplaceAllString(name, "_")
}

// GetWorktreePath returns the full path to the worktree
func (w *WorktreeInfo) GetWorktreePath(baseDir string) string {
	return filepath.Join(baseDir, w.Repo, w.WorktreeName)
}

// WorktreeListItem represents a single worktree in the list
type WorktreeListItem struct {
	Name        string
	Repo        string
	Branch      string
	Type        WorktreeType
	Path        string
	HasChanges  bool
	LastModTime int64
}

# AGENTS.md - Agentic Coding Guidelines

This document provides guidelines for agentic coding agents operating in this repository.

## Project Overview

`gh-worktree` is a GitHub CLI extension for managing git worktrees from GitHub PRs, Issues, and local branches. Built with Go 1.25.6 using cobra, viper, and go-gh.

## Build, Lint, and Test Commands

### Building the Project

```bash
# Build the extension binary
go build -o gh-worktree

# Or use taskfile (recommended)
task build
```

### Running the Extension

```bash
# Install locally as gh extension
task install

# Run the installed extension
task run [args]

# Build and run for development
task dev [args]
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a single test
go test -v -run TestFunctionName ./path/to/package

# Run tests with coverage
go test -cover ./...
```

**Note:** Currently no tests exist in this project. When adding tests, follow Go testing conventions.

### Linting and Formatting

```bash
# Format code (required before committing)
go fmt ./...

# Run go vet
go vet ./...

# Check for static analysis issues
go vet -shadow ./...
```

### Development Tasks

```bash
# Clean built binary
task clean

# Remove installed extension
task remove
```

## Code Style Guidelines

### General Principles

- Follow standard Go conventions (effectivego)
- Keep code simple and readable
- Use meaningful names that convey intent
- Write tests for new functionality

### Imports

Group imports in the following order with blank lines between groups:

1. Standard library packages
2. External/third-party packages
3. Internal packages (this project)

```go
import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/ffalor/gh-worktree/internal/config"
)
```

### Formatting

- Use `go fmt` for code formatting (enforced)
- Maximum line length: ~100 characters (soft limit)
- Use tabs for indentation, not spaces
- Use blank lines to separate logical sections within functions

### Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Packages | lowercase, short | `git`, `worktree`, `config` |
| Functions | PascalCase | `CreateWorktree`, `GetConfig` |
| Variables | camelCase | `worktreePath`, `forceFlag` |
| Constants | PascalCase | `DefaultWorktreeBase` |
| Interface types | PascalCase with -er suffix | `Reader`, `Creator` |
| Error variables | PascalCase with Err prefix | `ErrCancelled`, `ErrNotFound` |

### Types

- Use explicit types where helpful for documentation
- Use struct tags for serialization (e.g., JSON)
- Prefer concrete types over interfaces unless mocking is needed

### Error Handling

- Return errors with context using `fmt.Errorf("context: %w", err)`
- Use sentinel errors for known error conditions
- Handle errors at the appropriate level
- Avoid bare `panic()` except for unrecoverable conditions

```go
// Good error handling
if err != nil {
    return fmt.Errorf("failed to clone repository: %w", err)
}

// Sentinel error
var ErrCancelled = errors.New("cancelled")

// Check for sentinel errors
if errors.Is(err, ErrCancelled) {
    return nil
}
```

### Command Structure (Cobra)

- Use `cmd/` package for CLI command definitions
- Use `RunE` for commands that can fail
- Group related flags in `init()` functions
- Use persistent flags for global flags

```go
var createCmd = &cobra.Command{
    Use:   "create [url|name]",
    Short: "Create a new worktree",
    RunE:  runCreate,
}

func init() {
    rootCmd.AddCommand(createCmd)
    createCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "force operation")
}
```

### Logging and Output

- Use `fmt.Printf` for user-facing output
- Use appropriate verbosity levels for debug information
- Provide clear success/failure messages

### Configuration

- Use Viper for configuration management (see `internal/config`)
- Support config file, environment variables, and flags
- Provide sensible defaults

### GitHub CLI Integration

- Use `github.com/cli/go-gh/v2` for GitHub API calls
- Parse JSON output with `json.Unmarshal`
- Handle gh CLI errors gracefully

### Commit Messages

Use Conventional Commits format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Example: `feat(worktree): add support for GitHub issues`

## Project Structure

```
.
├── main.go              # Entry point
├── cmd/                 # CLI commands (cobra)
│   ├── root.go
│   ├── create.go
│   ├── list.go
│   └── remove.go
├── internal/
│   ├── config/          # Configuration management
│   ├── git/             # Git operations
│   └── worktree/        # Worktree logic
├── .agents/
│   └── skills/          # Agent skills (conventional-commits)
└── Taskfile.yml         # Development tasks
```

## Additional Notes

- This is a GitHub CLI extension - install with `gh extension install .`
- Requires `gh` CLI to be installed
- Worktrees are stored in `~/github/worktree` by default (configurable)
- Uses bare repositories for remote PR/Issue worktrees

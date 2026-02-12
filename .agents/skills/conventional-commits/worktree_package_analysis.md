  Analysis of Current State

  The worktree package, and specifically the Creator and setupWorktree function, is currently responsible for:


   1. Low-Level Git Orchestration: Calling functions from the git package (e.g., git.WorktreeAdd, git.Fetch). This is its core responsibility.
   2. Filesystem Operations: Directly creating and removing directories (os.MkdirAll, os.RemoveAll).
   3. Application-Specific Business Logic: Determining branch names based on the type of worktree (e.g., issue_%d, pr_%d).
   4. User Interface (UI) and Interaction: Printing status messages to the console (fmt.Printf) and handling interactive decisions via the branchCheck callback.
   5. State Management for Rollbacks: The Creator struct acts as a transaction manager to clean up resources if an operation fails.


  As you noted, the WorktreeInfo struct is a high-level concept used for creation, while Remove works with more primitive types (path, branch). This mismatch
  highlights the package's mixed responsibilities. The worktree package should not be concerned with where the data in WorktreeInfo comes from (e.g., GitHub
  API), nor should it be directly interacting with the user.

  Simplification Plan


  The goal is to refactor the worktree package into a pure, non-interactive library for managing Git worktrees. All application-specific logic, filesystem path
  generation, and user interaction should be moved to a higher level, specifically the cmd package.

  Here is a step-by-step plan:


  1. Redefine the `worktree` Package's Responsibility

   * The package's sole responsibility should be to provide a clean API that executes the necessary git worktree, git branch, and related Git commands.
   * Functions in this package should not print to stdout or handle user interaction. They should communicate outcomes by returning data and/or errors.

  2. Refactor the `Create` Operation


   * Eliminate the `Creator` struct. The transactional logic (cleanup on failure) can be handled more simply by the caller in the cmd package.
   * Replace (*Creator).Create with a simpler, stateless function. This new function should not accept the complex WorktreeInfo struct. Instead, it should take
     the specific, pre-determined strings it needs to operate.


       * Current: creator.Create(info *WorktreeInfo)
       * Proposed: worktree.Create(path string, branch string, startPoint string)
           * path: The absolute path where the worktree should be created.
           * branch: The exact name of the branch to create.
           * startPoint: The ref to start from (e.g., HEAD, FETCH_HEAD, an existing branch).

   * All logic for generating the path and branch names, fetching PRs, and handling the branchCheck interaction should be moved into cmd/create.go. The command
     will first gather all information and user input, then make a single, simple call to worktree.Create.


  3. Refactor the `Remove` Operation

   * The current Remove function is already more focused, but it combines two distinct actions: removing the worktree and deleting the branch.
   * For better separation of concerns, split this functionality.


       * `worktree.Remove(path string, force bool)`: This function's only job is to run git worktree remove and ensure the directory is gone from the disk. It
         should not be aware of or dependent on a branch name.
       * `git.BranchDelete(branch string, force bool)`: This function already exists and should be used directly.

   * The cmd/remove.go file will then be responsible for calling both worktree.Remove() and git.BranchDelete(), giving it full control over the removal process.

  4. Isolate UI and Business Logic


   * Remove all `fmt.Printf` calls from the worktree package. The cmd package should be responsible for reporting progress and results to the user.
   * The logic for constructing branch names (issue_%d) is business logic for gh-worktree, not a general worktree concept. This should live entirely within
     cmd/create.go.


  5. The Role of `WorktreeInfo`


   * WorktreeInfo remains a useful struct, but it should be used exclusively within the cmd layer. It will act as a data container to hold information fetched
     from APIs (like GitHub) before that data is processed into the simple arguments required by the newly refactored worktree and git package functions.

  Benefits of this Refactoring


   1. Improved Separation of Concerns: Each package will have a single, clear responsibility.
   2. Enhanced Testability: It is far easier to write unit tests for a pure library function like worktree.Create("path", "branch", "HEAD") than for a function
      that performs file I/O, prints to the console, and has interactive callbacks.
   3. Better Maintainability: The code will be easier to understand and modify, as logic for a single feature (e.g., creating a worktree) will be located in one
      place (cmd/create.go) rather than being split between the cmd and internal/worktree packages.
   4. Increased Reusability: A pure worktree library could potentially be used by other parts of the application or even other projects.
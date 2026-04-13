# wt v2: Navigator & Cleaner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Slim wt from a full worktree lifecycle manager to a worktree navigator and cleaner, delegating creation/config/memory to Claude Code's native worktree support.

**Architecture:** Remove `wt new`, `wt remove`, config sync, memory sync, and merge-back features. Rework remaining commands (`switch`, `list`, `prune`, `shell-init`) to use Claude Code's `.claude/worktrees/` location. Simplify `Manager` struct to derive paths from `RepoRoot` only.

**Tech Stack:** Go 1.24, cobra, charmbracelet/huh

---

### Task 1: Delete unused files and packages

Remove entire files that are no longer needed. This is pure deletion with no code changes.

**Files:**
- Delete: `cmd/wt/new.go`
- Delete: `cmd/wt/new_test.go`
- Delete: `cmd/wt/remove.go`
- Delete: `cmd/wt/config_changes.go`
- Delete: `internal/config/config.go`
- Delete: `internal/config/config_test.go`
- Delete: `internal/worktree/memory.go`
- Delete: `internal/worktree/memory_test.go`

- [ ] **Step 1: Delete command files**

```bash
git rm cmd/wt/new.go cmd/wt/new_test.go cmd/wt/remove.go cmd/wt/config_changes.go
```

- [ ] **Step 2: Delete config package**

```bash
git rm internal/config/config.go internal/config/config_test.go
rmdir internal/config
```

- [ ] **Step 3: Delete memory files**

```bash
git rm internal/worktree/memory.go internal/worktree/memory_test.go
```

- [ ] **Step 4: Verify the project still compiles (expect failures — they'll be fixed in subsequent tasks)**

```bash
go build ./... 2>&1 | head -30
```

Expected: compilation errors referencing deleted symbols (config.DefaultPaths, config.LoadGlobalConfig, mgr.Create, HandleConfigChanges, HandleMemoryChanges, etc.). This confirms we deleted the right things and identifies all callsites to update.

- [ ] **Step 5: Commit the deletions**

```bash
git add -A
git commit -m "refactor: delete unused files — new, remove, config, memory

Removing wt new, wt remove, config package, memory sync, and
config change handling. These features are now handled by Claude
Code's native worktree support."
```

---

### Task 2: Simplify Manager struct and strip removed methods from worktree.go

Rework the core `Manager` type and remove all methods related to creation, config sync, snapshots, and merge-back.

**Files:**
- Modify: `internal/worktree/worktree.go`
- Modify: `internal/worktree/worktree_test.go`

- [ ] **Step 1: Write tests for the new Manager**

Add tests for the simplified `NewManager` and `WorktreePath` that verify `.claude/worktrees/` paths. Add to the top of `internal/worktree/worktree_test.go` (after the existing `setupRepoWithRemote` helper):

```go
func TestNewManager(t *testing.T) {
	mgr := NewManager("/home/user/myrepo")
	if mgr.RepoRoot != "/home/user/myrepo" {
		t.Errorf("RepoRoot = %q, want %q", mgr.RepoRoot, "/home/user/myrepo")
	}
}

func TestWorktreePath(t *testing.T) {
	mgr := NewManager("/home/user/myrepo")
	got := mgr.WorktreePath("feature-auth")
	want := "/home/user/myrepo/.claude/worktrees/feature-auth"
	if got != want {
		t.Errorf("WorktreePath = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run the new tests to verify they fail**

```bash
go test -v ./internal/worktree -run 'TestNewManager|TestWorktreePath'
```

Expected: compilation errors because `NewManager` still takes two args.

- [ ] **Step 3: Rewrite the Manager struct, NewManager, and WorktreePath**

Replace the existing Manager struct and related methods in `internal/worktree/worktree.go`:

Old code (lines 14-46):
```go
var ErrWorktreeExists = errors.New("worktree already exists")
var ErrWorktreeNotFound = errors.New("worktree does not exist")
var ErrBranchNotFound = errors.New("branch does not exist")
var ErrBaseBranchNotFound = errors.New("base branch not found")

// Manager handles worktree operations for a repository
type Manager struct {
	RepoRoot     string
	RepoName     string
	WorktreeBase string
}

// NewManager creates a Manager for the repo at the given root
func NewManager(repoRoot, worktreeBase string) *Manager {
	return &Manager{
		RepoRoot:     repoRoot,
		RepoName:     GetRepoName(repoRoot),
		WorktreeBase: worktreeBase,
	}
}

// WorktreePath returns the path where a branch's worktree would be located
func (m *Manager) WorktreePath(branch string) string {
	return filepath.Join(m.WorktreeBase, m.RepoName, branch)
}

// SnapshotPath returns the path where a branch's file snapshots are stored.
/ Snapshots live as a sibling to the worktrees directory:
// WorktreeBase = ~/.local/share/wt/worktrees → snapshots at ~/.local/share/wt/snapshots/<repo>/<branch>
func (m *Manager) SnapshotPath(branch string) string {
	wtRoot := filepath.Dir(m.WorktreeBase) // ~/.local/share/wt
	return filepath.Join(wtRoot, "snapshots", m.RepoName, branch)
}
```

Replace with:
```go
var ErrWorktreeNotFound = errors.New("worktree does not exist")
var ErrBranchNotFound = errors.New("branch does not exist")

// Manager handles worktree operations for a repository.
type Manager struct {
	RepoRoot string
}

// NewManager creates a Manager for the repo at the given root.
func NewManager(repoRoot string) *Manager {
	return &Manager{
		RepoRoot: repoRoot,
	}
}

// WorktreePath returns the path where a worktree is located.
func (m *Manager) WorktreePath(name string) string {
	return filepath.Join(m.RepoRoot, ".claude", "worktrees", name)
}
```

Note: `ErrWorktreeExists` and `ErrBaseBranchNotFound` are removed (only used by `Create`). `SnapshotPath` is removed.

- [ ] **Step 4: Remove deleted methods and types from worktree.go**

Delete these functions and types (keep the remaining git helper methods and `List`/`Remove`/`Exists`/`WorktreeInfo`):

- `Create` (lines 144-200)
- `CopyFiles` (lines 261-294)
- `SaveSnapshot` (lines 299-330)
- `RemoveSnapshot` (lines 334-341)
- `copyFile` (lines 343-365)
- `copyDir` (lines 367-385)
- `FileChange` type (lines 388-391)
- `MergeStatus` type and constants (lines 394-401)
- `MergeResult` type (lines 404-407)
- `DetectChanges` (lines 411-446)
- `detectFileChange` (lines 448-480)
- `detectDirChanges` (lines 482-513)
- `MergeBack` (lines 528-561)
- `mergeThreeWay` (lines 563-584)

Also remove `FetchBranch` (lines 129-136) — only used by `Create`.
Keep these methods (all still used by prune or other commands):
- `Exists`, `BranchExists`, `RemoteBranchExists`, `BranchUpstream`
- `HasUncommittedChanges`, `HasUnpushedCommits`
- `DeleteBranch`, `FetchPrune`
- `List`, `Remove`
- `WorktreeInfo`

Clean up unused imports after deletion (`io`, `io/fs` if present, etc.).

- [ ] **Step 5: Update WorktreeInfo struct**

Old:
```go
type WorktreeInfo struct {
	Path   string
	Branch string
}
```

New:
```go
// WorktreeInfo holds information about a worktree.
type WorktreeInfo struct {
	Name   string // Directory name (e.g., "feature-auth")
	Branch string // Git branch (e.g., "worktree-feature-auth")
	Path   string // Full filesystem path
}
```

- [ ] **Step 6: Update List() to read from .claude/worktrees/ and populate Name**

Old:
```go
func (m *Manager) List() ([]WorktreeInfo, error) {
	repoWorktreeDir := filepath.Join(m.WorktreeBase, m.RepoName)

	entries, err := os.ReadDir(repoWorktreeDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var worktrees []WorktreeInfo
	for _, entry := range entries {
		if entry.IsDir() {
			wtPath := filepath.Join(repoWorktreeDir, entry.Name())
			worktrees = append(worktrees, WorktreeInfo{
				Path:   wtPath,
				Branch: entry.Name(),
			})
		}
	}

	return worktrees, nil
}
```

New:
```go
// List returns all worktrees in .claude/worktrees/ for this repo.
func (m *Manager) List() ([]WorktreeInfo, error) {
	wtDir := filepath.Join(m.RepoRoot, ".claude", "worktrees")

	entries, err := os.ReadDir(wtDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var worktrees []WorktreeInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		wtPath := filepath.Join(wtDir, name)
		branch := branchForWorktree(wtPath)
		worktrees = append(worktrees, WorktreeInfo{
			Name:   name,
			Branch: branch,
			Path:   wtPath,
		})
	}

	return worktrees, nil
}

// branchForWorktree reads the branch checked out in a worktree.
func branchForWorktree(wtPath string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = wtPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
```

This reads the actual branch from each worktree via git, rather than assuming the `worktree-<name>` convention.

- [ ] **Step 7: Update Exists() to use the new WorktreePath**

`Exists` currently calls `m.WorktreePath(branch)` — the parameter name should change to `name` since we now use worktree names, not branch names. The body stays the same since `WorktreePath` handles the path change:

```go
// Exists checks if a worktree with the given name exists.
func (m *Manager) Exists(name string) bool {
	wtPath := m.WorktreePath(name)
	_, err := os.Stat(wtPath)
	return err == nil
}
```

- [ ] **Step 8: Update Remove() to use name and force-delete branch**

Old:
```go
func (m *Manager) Remove(branch string, force bool) error {
	wtPath := m.WorktreePath(branch)
	if !m.Exists(branch) {
		return ErrWorktreeNotFound
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wtPath)
	cmd := exec.Command("git", args...)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
```

New:
```go
// Remove removes a worktree by name.
// If force is true, removes even if worktree has uncommitted changes.
// Also deletes the associated local branch.
func (m *Manager) Remove(name string, force bool) error {
	wtPath := m.WorktreePath(name)

	if !m.Exists(name) {
		return ErrWorktreeNotFound
	}

	// Read the branch name before removing the worktree
	branch := branchForWorktree(wtPath)

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wtPath)

	cmd := exec.Command("git", args...)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}

	// Delete the local branch (force because remote may be gone)
	if branch != "" {
		_ = m.DeleteBranch(branch, true)
	}

	return nil
}
```

- [ ] **Step 9: Delete tests for removed functionality**

Remove these test functions from `internal/worktree/worktree_test.go`:
- `TestCreateWorktreeForRemoteBranch`
- `TestFetchBranch`
- `TestFetchBranchNotFound`
- `TestCreateWithBaseBranch`
- `TestCreateWithRemoteBaseBranch`
- `TestCreateWithBaseBranchNotFound`
- `TestCreateWithEmptyBaseBranch`
- `TestCopyFiles_CopiesDirectory`
- `TestDetectChanges_DetectsDirectoryChanges`
- `TestDetectChanges_DetectsNewFileInDirectory`
- `TestMergeBack_MergesDirectory`
- `TestSnapshotPath`
- `TestSaveSnapshot`
- `TestSaveSnapshot_SkipsNonexistent`
- `TestRemoveSnapshot`
- `TestRemoveSnapshot_NonexistentIsNotError`
- `TestMergeBack_ThreeWayCleanMerge`
- `TestMergeBack_ThreeWayConflict`
- `TestMergeBack_FallbackNoSnapshot`
- `TestMergeBack_DirectoryCopy`

- [ ] **Step 10: Update setupRepoWithRemote helper**

The test helper currently returns `worktreeBase` as a separate temp dir. Update it to create `.claude/worktrees/` inside the main repo instead:

Old:
```go
func setupRepoWithRemote(t *testing.T) (mainRepo, bareRemote, worktreeBase string) {
	t.Helper()
	tmpDir := t.TempDir()
	bareRemote = filepath.Join(tmpDir, "remote.git")
	mainRepo = filepath.Join(tmpDir, "local")
	worktreeBase = filepath.Join(tmpDir, "worktrees")
	// ... creates bare remote and clones
```

New:
```go
func setupRepoWithRemote(t *testing.T) (mainRepo, bareRemote string) {
	t.Helper()
	tmpDir := t.TempDir()
	bareRemote = filepath.Join(tmpDir, "remote.git")
	mainRepo = filepath.Join(tmpDir, "local")
	// ... creates bare remote and clones (same as before, just drop worktreeBase)
```

The `.claude/worktrees/` directory is derived from `mainRepo` by `NewManager`, so tests don't need to pass it.

- [ ] **Step 11: Update remaining tests to use new Manager constructor**

All test calls of `NewManager(repoRoot, worktreeBase)` become `NewManager(repoRoot)`. All references to `wt.Branch` in test assertions become `wt.Name`. Update all callers in `worktree_test.go`.

- [ ] **Step 12: Run tests**

```bash
go test -v ./internal/worktree/...
```

Expected: all tests pass.

- [ ] **Step 13: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "refactor: simplify Manager to use .claude/worktrees/ paths

Remove creation, config sync, snapshot, and merge-back methods.
WorktreeInfo is now name-primary. List() reads branch from git.
Remove() also deletes the associated local branch."
```

---

### Task 3: Update repo.go — remove GetRepoName

`GetRepoName` was only used by the old `Manager` to build XDG paths. It's no longer needed.

**Files:**
- Modify: `internal/worktree/repo.go`
- Modify: `internal/worktree/repo_test.go`

- [ ] **Step 1: Check if GetRepoName is used anywhere**

```bash
grep -r 'GetRepoName' --include='*.go' .
```

Expected: only `repo.go` definition and possibly old test references.

- [ ] **Step 2: Remove GetRepoName from repo.go**

Delete:
```go
// GetRepoName extracts the repository name from its path.
func GetRepoName(repoRoot string) string {
	return filepath.Base(repoRoot)
}
```

- [ ] **Step 3: Remove any GetRepoName tests from repo_test.go**

- [ ] **Step 4: Run tests**

```bash
go test -v ./internal/worktree/...
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add internal/worktree/repo.go internal/worktree/repo_test.go
git commit -m "refactor: remove GetRepoName — no longer needed without XDG paths"
```

---

### Task 4: Update list command

**Files:**
- Modify: `cmd/wt/list.go`

- [ ] **Step 1: Rewrite list.go**

The current command uses `config.DefaultPaths()` and passes `worktreeBase`. Replace with the simplified Manager:

```go
package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List worktrees for current repo",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		mgr := worktree.NewManager(repoRoot)
		worktrees, err := mgr.List()
		if err != nil {
			return err
		}

		if len(worktrees) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found")
			return nil
		}

		for _, wt := range worktrees {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", wt.Name, wt.Branch, wt.Path)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
```

Note: the `--worktree-base` flag is removed since the path is now derived.

- [ ] **Step 2: Verify it compiles**

```bash
go build ./cmd/wt/...
```

Expected: pass (once switch/prune are also updated — may still fail if those aren't done yet).

- [ ] **Step 3: Commit**

```bash
git add cmd/wt/list.go
git commit -m "refactor: update list command for .claude/worktrees/ paths

Remove --worktree-base flag and config dependency."
```

---

### Task 5: Update switch command

**Files:**
- Modify: `cmd/wt/switch.go`
- Modify: `cmd/wt/picker.go`

- [ ] **Step 1: Rewrite switch.go**

The switch command simplifies significantly — no more config loading, no auto-creating worktrees, no `CopyFiles`. It just navigates to existing worktrees:

```go
package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var switchPrintPath bool

var switchCmd = &cobra.Command{
	Use:   "switch [name]",
	Short: "Switch to a worktree",
	Long: `Switch to a worktree by name. The name is the directory name under .claude/worktrees/.

If no name is specified, displays an interactive picker to select from available worktrees.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		mgr := worktree.NewManager(repoRoot)

		// Determine name — either from argument or interactive picker
		var name string
		if len(args) == 0 {
			name, err = runInteractivePicker(repoRoot, mgr)
			if err != nil {
				return err
			}
		} else {
			name = args[0]
		}

		// "main" means the repo root itself
		mainBranch, err := worktree.GetMainBranch(repoRoot)
		if err == nil && name == mainBranch {
			if switchPrintPath {
				fmt.Fprintln(cmd.OutOrStdout(), repoRoot)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Switched to %s\n", repoRoot)
			}
			return nil
		}

		if !mgr.Exists(name) {
			return fmt.Errorf("worktree %q does not exist (use 'claude --worktree %s' to create it)", name, name)
		}

		wtPath := mgr.WorktreePath(name)
		if switchPrintPath {
			fmt.Fprintln(cmd.OutOrStdout(), wtPath)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Switched to %s\n", wtPath)
		}

		return nil
	},
}

func init() {
	switchCmd.Flags().BoolVar(&switchPrintPath, "print-path", false, "Only print the worktree path")
	rootCmd.AddCommand(switchCmd)
}
```

- [ ] **Step 2: Update picker.go to use Name instead of Branch**

```go
package main

import (
	"github.com/charmbracelet/huh"
	"github.com/niref/wt/internal/worktree"
)

// buildPickerOptions creates the list of options for the interactive picker.
// The main branch is always listed first, followed by worktree names.
func buildPickerOptions(mainBranch string, worktrees []worktree.WorktreeInfo) []string {
	options := make([]string, 0, 1+len(worktrees))
	options = append(options, mainBranch)
	for _, wt := range worktrees {
		options = append(options, wt.Name)
	}
	return options
}

// runInteractivePicker displays an interactive picker and returns the selected name.
func runInteractivePicker(repoRoot string, mgr *worktree.Manager) (string, error) {
	mainBranch, err := worktree.GetMainBranch(repoRoot)
	if err != nil {
		return "", err
	}

	worktrees, err := mgr.List()
	if err != nil {
		return "", err
	}

	options := buildPickerOptions(mainBranch, worktrees)

	huhOptions := make([]huh.Option[string], len(options))
	for i, opt := range options {
		huhOptions[i] = huh.NewOption(opt, opt)
	}

	var selected string
	err = huh.NewSelect[string]().
		Title("Select worktree").
		Options(huhOptions...).
		Value(&selected).
		Run()

	if err != nil {
		return "", err
	}

	return selected, nil
}
```

- [ ] **Step 3: Update picker_test.go**

Update `TestBuildPickerOptions` (if it exists) to use `WorktreeInfo{Name: "feat", ...}` instead of `WorktreeInfo{Branch: "feat", ...}`.

- [ ] **Step 4: Verify it compiles**

```bash
go build ./cmd/wt/...
```

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/switch.go cmd/wt/picker.go cmd/wt/picker_test.go
git commit -m "refactor: simplify switch to navigate existing worktrees only

No more auto-creation. Uses worktree name instead of branch.
Removes config dependency and --worktree-base/--config flags."
```

---

### Task 6: Update prune command

**Files:**
- Modify: `cmd/wt/prune.go`
- Modify: `cmd/wt/prune_test.go`

- [ ] **Step 1: Rewrite prune.go**

The prune command loses config/memory change detection and gains proper force-delete. It still checks for uncommitted changes and unpushed commits:

```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	pruneForce  bool
	pruneNoFetch bool
	pruneDryRun bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove worktrees for branches deleted from remote",
	Long: `Remove worktrees whose branches have been deleted from the remote (merged or manually deleted).

Only considers branches with upstream tracking configured - local-only branches are never pruned.
Use --dry-run to preview what would be removed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		mgr := worktree.NewManager(repoRoot)

		// Fetch and prune remote refs (unless --no-fetch)
		if !pruneNoFetch {
			if err := mgr.FetchPrune(); err != nil {
				return fmt.Errorf("fetch failed: %w\nUse --no-fetch to skip fetching", err)
			}
		}

		// Get all worktrees
		worktrees, err := mgr.List()
		if err != nil {
			return err
		}

		if len(worktrees) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found")
			return nil
		}

		// Find prune candidates: worktrees whose branch has upstream tracking
		// but the remote branch no longer exists
		var candidates []worktree.WorktreeInfo
		for _, wt := range worktrees {
			if wt.Branch == "" {
				continue
			}
			upstream := mgr.BranchUpstream(wt.Branch)
			if upstream == "" {
				// No upstream tracking - skip (local-only branch)
				continue
			}
			if mgr.RemoteBranchExists(wt.Branch) {
				// Remote branch still exists - not a prune candidate
				continue
			}
			candidates = append(candidates, wt)
		}

		if len(candidates) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune")
			return nil
		}

		// Dry-run mode
		if pruneDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "Would prune (dry-run):")
			for _, c := range candidates {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s)\n", c.Name, c.Branch)
			}
			return nil
		}

		// Prune each candidate
		var pruned []string
		var errors []string

		for _, candidate := range candidates {
			name := candidate.Name
			wtPath := candidate.Path

			// Check for issues that require prompting
			hasUncommitted := mgr.HasUncommittedChanges(wtPath)
			hasUnpushed := mgr.HasUnpushedCommits(candidate.Branch)

			if (hasUncommitted || hasUnpushed) && !pruneForce {
				issues := []string{}
				if hasUncommitted {
					issues = append(issues, "uncommitted changes")
				}
				if hasUnpushed {
					issues = append(issues, "unpushed commits")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Remove %s? It has %s [y/n]: ", name, strings.Join(issues, " and "))

				reader := bufio.NewReader(os.Stdin)
				input, err := reader.ReadString('\n')
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s: failed to read input: %v", name, err))
					continue
				}
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "y" && input != "yes" {
					fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s\n", name)
					continue
				}
			}

			// Remove worktree (force: true because user confirmed or --force flag)
			if err := mgr.Remove(name, true); err != nil {
				errors = append(errors, fmt.Sprintf("%s: remove worktree: %v", name, err))
				continue
			}

			pruned = append(pruned, name)
		}

		// Print summary
		if len(pruned) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d worktree(s):\n", len(pruned))
			for _, p := range pruned {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
			}
		}

		if len(errors) > 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "\nErrors:")
			for _, e := range errors {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", e)
			}
		}

		if len(pruned) == 0 && len(errors) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune")
		}

		return nil
	},
}

func init() {
	pruneCmd.Flags().BoolVarP(&pruneForce, "force", "f", false, "Force removal even if worktrees have uncommitted changes")
	pruneCmd.Flags().BoolVar(&pruneNoFetch, "no-fetch", false, "Skip git fetch --prune (use current remote refs)")
	pruneCmd.Flags().BoolVarP(&pruneDryRun, "dry-run", "n", false, "Show what would be pruned without doing it")
	rootCmd.AddCommand(pruneCmd)
}
```

Key changes:
- No more config/memory change detection blocks
- No more snapshot cleanup
- `mgr.Remove(name, true)` always force-removes (user already confirmed or `--force` flag is set)
- `Remove()` now handles branch deletion internally
- Removed `--worktree-base`, `--config`, and `--skip-changes` flags

- [ ] **Step 2: Update prune_test.go**

Update existing tests to:
- Use `NewManager(repoRoot)` instead of `NewManager(repoRoot, worktreeBase)`
- Create worktrees in `.claude/worktrees/` inside the test repo instead of external dir
- Reference `wt.Name` instead of `wt.Branch` where appropriate
- Remove any tests that exercise config/memory change detection

The test helper for prune tests likely sets up worktrees via `git worktree add` — update the paths to be under `.claude/worktrees/` in the test repo.

- [ ] **Step 3: Run tests**

```bash
go test -v ./cmd/wt/...
```

Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/wt/prune.go cmd/wt/prune_test.go
git commit -m "refactor: simplify prune — remove config/memory change detection

Force-deletes worktree and branch after user confirmation.
Removes --worktree-base, --config, and --skip-changes flags."
```

---

### Task 7: Update shell integration

**Files:**
- Modify: `internal/shell/shell.go`
- Modify: `internal/shell/shell_test.go`

- [ ] **Step 1: Update bashInit to remove wt new wrapper**

The shell function currently handles `new|switch` cases. Remove the `new` case:

Old:
```go
const bashInit = `# wt shell integration
# Add to your ~/.bashrc or ~/.zshrc:
#   eval "$(wt-bin shell-init bash)"

wt() {
    case "$1" in
        new|switch)
            # Check if this might be interactive (switch with no branch argument)
            ...
```

New:
```go
const bashInit = `# wt shell integration
# Add to your ~/.bashrc or ~/.zshrc:
#   eval "$(wt-bin shell-init bash)"

wt() {
    case "$1" in
        switch)
            # Check if this might be interactive (switch with no argument)
            # Interactive mode needs direct TTY access, so we can't use command substitution
            if [ $# -eq 1 ]; then
                # Interactive mode: run directly, write path to temp file
                local tmpfile
                tmpfile=$(mktemp)
                wt-bin switch --print-path > "$tmpfile"
                local exit_code=$?
                if [ $exit_code -eq 0 ]; then
                    local output
                    output=$(cat "$tmpfile")
                    rm -f "$tmpfile"
                    if [ -d "$output" ]; then
                        cd "$output"
                    fi
                else
                    rm -f "$tmpfile"
                    return $exit_code
                fi
            else
                # Non-interactive: can safely capture output
                local output
                output=$(wt-bin "$@" --print-path 2>&1)
                local exit_code=$?
                if [ $exit_code -eq 0 ] && [ -d "$output" ]; then
                    cd "$output"
                else
                    echo "$output" >&2
                    return $exit_code
                fi
            fi
            ;;
        *)
            wt-bin "$@"
            ;;
    esac
}
`
```

- [ ] **Step 2: Update shell tests**

Update any tests that assert `new|switch` in the output to assert `switch)` only.

- [ ] **Step 3: Run tests**

```bash
go test -v ./internal/shell/...
```

Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add internal/shell/shell.go internal/shell/shell_test.go
git commit -m "refactor: remove wt new from shell integration

Only switch needs the cd wrapper now."
```

---

### Task 8: Update main.go and remove toml dependency

**Files:**
- Modify: `cmd/wt/main.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Update main.go description**

```go
var rootCmd = &cobra.Command{
	Use:   "wt-bin",
	Short: "Git worktree navigator and cleaner",
}
```

- [ ] **Step 2: Remove toml dependency**

```bash
go mod tidy
```

This should remove `github.com/pelletier/go-toml/v2` since nothing imports `internal/config` anymore.

- [ ] **Step 3: Verify clean build**

```bash
go build -o wt-bin ./cmd/wt
```

Expected: builds successfully.

- [ ] **Step 4: Run all tests**

```bash
go test -v ./...
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/wt/main.go go.mod go.sum
git commit -m "chore: update description and remove toml dependency"
```

---

### Task 9: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md to reflect new architecture**

Update the Project Overview, Architecture, and Key Design Decisions sections:

- Project Overview: change "manages git worktrees with automatic config file copying and Podman container support" to "navigates and cleans up git worktrees created by Claude Code, with optional Podman container support"
- Architecture: remove references to config package, memory.go, config file copying, snapshot/merge-back
- Key Design Decisions: remove "File change detection", "Memory sync", update "XDG compliance" → worktrees now in `.claude/worktrees/`
- Remove config merging documentation
- Update error handling: remove `ErrWorktreeExists` (deleted)

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for v2 architecture"
```

---

### Task 10: Final verification

- [ ] **Step 1: Run full test suite**

```bash
go test -v ./...
```

Expected: all pass, no references to deleted symbols.

- [ ] **Step 2: Build and smoke test**

```bash
go build -o wt-bin ./cmd/wt
./wt-bin list
./wt-bin --help
./wt-bin switch --help
./wt-bin prune --help
```

Verify help text is correct and list works (may show "No worktrees found" if no `.claude/worktrees/` exists — that's fine).

- [ ] **Step 3: Verify no stale references**

```bash
grep -r 'config\.DefaultPaths\|config\.LoadGlobal\|config\.MergeConfigs\|CopyFiles\|SaveSnapshot\|DetectChanges\|MergeBack\|HandleConfigChanges\|HandleMemoryChanges\|DetectMemoryChanges\|RemoveMemorySnapshot\|ClaudeMemoryDir\|CopyMemory\|MergeMemoryBack' --include='*.go' .
```

Expected: no matches.

- [ ] **Step 4: Verify no unused imports**

```bash
go vet ./...
```

Expected: clean.

- [ ] **Step 5: Commit any final fixes if needed**

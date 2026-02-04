# `wt prune` Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `wt prune` command that removes worktrees for branches deleted from remote.

**Architecture:** New Manager methods for git state inspection, new Cobra command that orchestrates the prune algorithm with per-worktree prompts.

**Tech Stack:** Go, Cobra CLI, git plumbing commands

---

## Task 1: Add `BranchUpstream` method

**Files:**
- Modify: `internal/worktree/worktree.go`
- Test: `internal/worktree/worktree_test.go`

**Step 1: Write the failing test**

Add to `internal/worktree/worktree_test.go`:

```go
func TestBranchUpstream_WithTracking(t *testing.T) {
	mainRepo, bareRemote, worktreeBase := setupRepoWithRemote(t)

	// Create and push a branch with tracking
	cmds := [][]string{
		{"git", "checkout", "-b", "tracked-branch"},
		{"git", "commit", "--allow-empty", "-m", "tracked commit"},
		{"git", "push", "-u", "origin", "tracked-branch"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = mainRepo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
	_ = bareRemote // used by setup

	mgr := NewManager(mainRepo, worktreeBase)

	upstream := mgr.BranchUpstream("tracked-branch")
	if upstream != "origin/tracked-branch" {
		t.Errorf("expected 'origin/tracked-branch', got %q", upstream)
	}
}

func TestBranchUpstream_NoTracking(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)

	// Create a local-only branch (no push, no tracking)
	cmd := exec.Command("git", "checkout", "-b", "local-only")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo, worktreeBase)

	upstream := mgr.BranchUpstream("local-only")
	if upstream != "" {
		t.Errorf("expected empty string for local-only branch, got %q", upstream)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worktree -run TestBranchUpstream`
Expected: FAIL with "BranchUpstream not defined" or similar

**Step 3: Write minimal implementation**

Add to `internal/worktree/worktree.go` after `RemoteBranchExists`:

```go
// BranchUpstream returns the upstream tracking ref for a branch (e.g., "origin/main").
// Returns empty string if the branch has no upstream configured.
func (m *Manager) BranchUpstream(branch string) string {
	cmd := exec.Command("git", "for-each-ref", "--format=%(upstream:short)", "refs/heads/"+branch)
	cmd.Dir = m.RepoRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/worktree -run TestBranchUpstream`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "feat(worktree): add BranchUpstream method"
```

---

## Task 2: Add `DeleteBranch` method

**Files:**
- Modify: `internal/worktree/worktree.go`
- Test: `internal/worktree/worktree_test.go`

**Step 1: Write the failing test**

Add to `internal/worktree/worktree_test.go`:

```go
func TestDeleteBranch(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)

	// Create a branch
	cmd := exec.Command("git", "branch", "branch-to-delete")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create branch failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo, worktreeBase)

	// Verify branch exists
	if !mgr.BranchExists("branch-to-delete") {
		t.Fatal("branch should exist before delete")
	}

	// Delete the branch
	err := mgr.DeleteBranch("branch-to-delete")
	if err != nil {
		t.Fatalf("DeleteBranch failed: %v", err)
	}

	// Verify branch is gone
	if mgr.BranchExists("branch-to-delete") {
		t.Error("branch should not exist after delete")
	}
}

func TestDeleteBranch_NotFound(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	err := mgr.DeleteBranch("nonexistent-branch")
	if err == nil {
		t.Error("expected error for nonexistent branch")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worktree -run TestDeleteBranch`
Expected: FAIL

**Step 3: Write minimal implementation**

Add to `internal/worktree/worktree.go`:

```go
// DeleteBranch deletes a local branch.
// Returns an error if the branch doesn't exist or can't be deleted.
func (m *Manager) DeleteBranch(branch string) error {
	cmd := exec.Command("git", "branch", "-d", branch)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch -d: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/worktree -run TestDeleteBranch`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "feat(worktree): add DeleteBranch method"
```

---

## Task 3: Add `HasUncommittedChanges` method

**Files:**
- Modify: `internal/worktree/worktree.go`
- Test: `internal/worktree/worktree_test.go`

**Step 1: Write the failing test**

Add to `internal/worktree/worktree_test.go`:

```go
func TestHasUncommittedChanges_Clean(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	// Create a clean worktree
	wtPath, err := mgr.Create("clean-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if mgr.HasUncommittedChanges(wtPath) {
		t.Error("clean worktree should not have uncommitted changes")
	}
}

func TestHasUncommittedChanges_Modified(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	// Create worktree
	wtPath, err := mgr.Create("modified-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Add an untracked file
	testFile := filepath.Join(wtPath, "untracked.txt")
	if err := os.WriteFile(testFile, []byte("untracked"), 0644); err != nil {
		t.Fatal(err)
	}

	if !mgr.HasUncommittedChanges(wtPath) {
		t.Error("worktree with untracked file should have uncommitted changes")
	}
}

func TestHasUncommittedChanges_Staged(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	// Create worktree
	wtPath, err := mgr.Create("staged-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create and stage a file
	testFile := filepath.Join(wtPath, "staged.txt")
	if err := os.WriteFile(testFile, []byte("staged"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "staged.txt")
	cmd.Dir = wtPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	if !mgr.HasUncommittedChanges(wtPath) {
		t.Error("worktree with staged file should have uncommitted changes")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worktree -run TestHasUncommittedChanges`
Expected: FAIL

**Step 3: Write minimal implementation**

Add to `internal/worktree/worktree.go`:

```go
// HasUncommittedChanges checks if a worktree has uncommitted changes.
// This includes untracked files, modified files, and staged changes.
func (m *Manager) HasUncommittedChanges(wtPath string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = wtPath
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/worktree -run TestHasUncommittedChanges`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "feat(worktree): add HasUncommittedChanges method"
```

---

## Task 4: Add `HasUnpushedCommits` method

**Files:**
- Modify: `internal/worktree/worktree.go`
- Test: `internal/worktree/worktree_test.go`

**Step 1: Write the failing test**

Add to `internal/worktree/worktree_test.go`:

```go
func TestHasUnpushedCommits_NoneAhead(t *testing.T) {
	mainRepo, bareRemote, worktreeBase := setupRepoWithRemote(t)

	// Create and push a branch (fully synced)
	cmds := [][]string{
		{"git", "checkout", "-b", "synced-branch"},
		{"git", "commit", "--allow-empty", "-m", "synced commit"},
		{"git", "push", "-u", "origin", "synced-branch"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = mainRepo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
	_ = bareRemote

	mgr := NewManager(mainRepo, worktreeBase)

	if mgr.HasUnpushedCommits("synced-branch") {
		t.Error("fully synced branch should not have unpushed commits")
	}
}

func TestHasUnpushedCommits_Ahead(t *testing.T) {
	mainRepo, bareRemote, worktreeBase := setupRepoWithRemote(t)

	// Create and push a branch, then add local commit
	cmds := [][]string{
		{"git", "checkout", "-b", "ahead-branch"},
		{"git", "commit", "--allow-empty", "-m", "pushed commit"},
		{"git", "push", "-u", "origin", "ahead-branch"},
		{"git", "commit", "--allow-empty", "-m", "local only commit"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = mainRepo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
	_ = bareRemote

	mgr := NewManager(mainRepo, worktreeBase)

	if !mgr.HasUnpushedCommits("ahead-branch") {
		t.Error("branch with local commit should have unpushed commits")
	}
}

func TestHasUnpushedCommits_NoUpstream(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)

	// Create local-only branch (no upstream)
	cmd := exec.Command("git", "checkout", "-b", "no-upstream")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo, worktreeBase)

	// No upstream means we can't determine - treat as "no unpushed" for prune safety
	// (branches without upstream won't be pruned anyway)
	if mgr.HasUnpushedCommits("no-upstream") {
		t.Error("branch with no upstream should return false")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worktree -run TestHasUnpushedCommits`
Expected: FAIL

**Step 3: Write minimal implementation**

Add to `internal/worktree/worktree.go`:

```go
// HasUnpushedCommits checks if a branch has commits not pushed to its upstream.
// Returns false if the branch has no upstream configured.
func (m *Manager) HasUnpushedCommits(branch string) bool {
	upstream := m.BranchUpstream(branch)
	if upstream == "" {
		return false
	}
	// Count commits in branch that are not in upstream
	cmd := exec.Command("git", "rev-list", "--count", upstream+".."+branch)
	cmd.Dir = m.RepoRoot
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	count := strings.TrimSpace(string(out))
	return count != "0"
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/worktree -run TestHasUnpushedCommits`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "feat(worktree): add HasUnpushedCommits method"
```

---

## Task 5: Add `FetchPrune` method

**Files:**
- Modify: `internal/worktree/worktree.go`
- Test: `internal/worktree/worktree_test.go`

**Step 1: Write the failing test**

Add to `internal/worktree/worktree_test.go`:

```go
func TestFetchPrune(t *testing.T) {
	mainRepo, bareRemote, worktreeBase := setupRepoWithRemote(t)

	// Create a branch from another clone and push it
	tmpClone := filepath.Join(t.TempDir(), "tmpclone")
	cmd := exec.Command("git", "clone", bareRemote, tmpClone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}

	cmds := [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "to-be-deleted"},
		{"git", "commit", "--allow-empty", "-m", "temp commit"},
		{"git", "push", "-u", "origin", "to-be-deleted"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpClone
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Fetch in main repo so it knows about the branch
	cmd = exec.Command("git", "fetch", "origin")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fetch failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo, worktreeBase)

	// Verify remote branch is known
	if !mgr.RemoteBranchExists("to-be-deleted") {
		t.Fatal("remote branch should exist before delete")
	}

	// Delete the branch on remote
	cmd = exec.Command("git", "push", "origin", "--delete", "to-be-deleted")
	cmd.Dir = tmpClone
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("push delete failed: %v\n%s", err, out)
	}

	// Without fetch --prune, main repo still thinks branch exists
	if !mgr.RemoteBranchExists("to-be-deleted") {
		t.Fatal("stale remote ref should still exist before FetchPrune")
	}

	// Run FetchPrune
	err := mgr.FetchPrune()
	if err != nil {
		t.Fatalf("FetchPrune failed: %v", err)
	}

	// Now the stale remote ref should be gone
	if mgr.RemoteBranchExists("to-be-deleted") {
		t.Error("remote ref should be pruned after FetchPrune")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worktree -run TestFetchPrune`
Expected: FAIL

**Step 3: Write minimal implementation**

Add to `internal/worktree/worktree.go`:

```go
// FetchPrune fetches from origin and prunes stale remote-tracking refs.
func (m *Manager) FetchPrune() error {
	cmd := exec.Command("git", "fetch", "--prune", "origin")
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch --prune origin: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/worktree -run TestFetchPrune`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "feat(worktree): add FetchPrune method"
```

---

## Task 6: Create basic `wt prune` command with dry-run

**Files:**
- Create: `cmd/wt/prune.go`
- Test: `cmd/wt/prune_test.go`

**Step 1: Write the failing test**

Create `cmd/wt/prune_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niref/wt/internal/worktree"
)

func TestPrune_DryRun_ShowsCandidates(t *testing.T) {
	repoDir, worktreeBase, bareRemote := setupTestRepoWithRemote(t)

	// Create a branch, push it, create worktree, then delete from remote
	cmds := [][]string{
		{"git", "checkout", "-b", "gone-branch"},
		{"git", "commit", "--allow-empty", "-m", "gone commit"},
		{"git", "push", "-u", "origin", "gone-branch"},
		{"git", "checkout", "master"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Create worktree for the branch
	mgr := worktree.NewManager(repoDir, worktreeBase)
	_, err := mgr.Create("gone-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete branch from remote (simulating merge)
	cmd := exec.Command("git", "push", "origin", "--delete", "gone-branch")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("push delete failed: %v\n%s", err, out)
	}
	_ = bareRemote

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		pruneDryRun = false
		pruneNoFetch = false
	}()

	rootCmd.SetArgs([]string{"prune", "--dry-run",
		"--worktree-base", worktreeBase,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune --dry-run failed: %v\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "gone-branch") {
		t.Errorf("output should mention gone-branch, got: %s", output)
	}
	if !strings.Contains(output, "Would prune") || !strings.Contains(output, "dry-run") {
		t.Errorf("output should indicate dry-run mode, got: %s", output)
	}

	// Worktree should still exist (dry-run)
	if !mgr.Exists("gone-branch") {
		t.Error("worktree should still exist after dry-run")
	}
}

func TestPrune_NothingToPrune(t *testing.T) {
	repoDir, worktreeBase, _ := setupTestRepoWithRemote(t)

	// Create a branch that still exists on remote
	cmds := [][]string{
		{"git", "checkout", "-b", "active-branch"},
		{"git", "commit", "--allow-empty", "-m", "active commit"},
		{"git", "push", "-u", "origin", "active-branch"},
		{"git", "checkout", "master"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Create worktree
	mgr := worktree.NewManager(repoDir, worktreeBase)
	_, err := mgr.Create("active-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		pruneDryRun = false
		pruneNoFetch = false
	}()

	rootCmd.SetArgs([]string{"prune",
		"--worktree-base", worktreeBase,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune failed: %v\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "Nothing to prune") {
		t.Errorf("output should say nothing to prune, got: %s", output)
	}
}

func TestPrune_SkipsLocalOnlyBranches(t *testing.T) {
	repoDir, worktreeBase, _ := setupTestRepoWithRemote(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create a local-only branch (never pushed, no tracking)
	mgr := worktree.NewManager(repoDir, worktreeBase)
	_, err := mgr.Create("local-only-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		pruneDryRun = false
		pruneNoFetch = false
	}()

	rootCmd.SetArgs([]string{"prune", "--dry-run",
		"--worktree-base", worktreeBase,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune failed: %v\n%s", err, buf.String())
	}

	output := buf.String()
	// Should NOT mention local-only-branch as a prune candidate
	if strings.Contains(output, "local-only-branch") && !strings.Contains(output, "Nothing to prune") {
		t.Errorf("local-only branch should not be a prune candidate, got: %s", output)
	}

	// Worktree should still exist
	if !mgr.Exists("local-only-branch") {
		t.Error("local-only worktree should not be pruned")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./cmd/wt -run TestPrune`
Expected: FAIL (prune command doesn't exist)

**Step 3: Write minimal implementation**

Create `cmd/wt/prune.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	pruneWorktreeBase string
	pruneConfigPath   string
	pruneForce        bool
	pruneSkipChanges  bool
	pruneNoFetch      bool
	pruneDryRun       bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove worktrees for branches deleted from remote",
	Long: `Remove worktrees whose branches have been deleted from the remote (merged or manually deleted).

Only considers branches with upstream tracking configured - local-only branches are never pruned.
Prompts for worktrees with uncommitted changes or config file modifications.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		paths := config.DefaultPaths()
		wtBase := pruneWorktreeBase
		if wtBase == "" {
			wtBase = paths.WorktreeBase
		}

		mgr := worktree.NewManager(repoRoot, wtBase)

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

		// Find prune candidates
		var candidates []worktree.WorktreeInfo
		for _, wt := range worktrees {
			upstream := mgr.BranchUpstream(wt.Branch)
			if upstream == "" {
				// No upstream tracking - skip (local-only branch)
				continue
			}
			// Check if upstream remote ref still exists
			// upstream is like "origin/branch-name", extract branch name
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
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", c.Branch)
			}
			return nil
		}

		// TODO: Implement actual pruning with prompts (Task 7)
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune")
		return nil
	},
}

func init() {
	pruneCmd.Flags().StringVar(&pruneWorktreeBase, "worktree-base", "", "Override worktree base directory")
	pruneCmd.Flags().StringVar(&pruneConfigPath, "config", "", "Override global config path")
	pruneCmd.Flags().BoolVarP(&pruneForce, "force", "f", false, "Force removal even if worktrees have uncommitted changes")
	pruneCmd.Flags().BoolVar(&pruneSkipChanges, "skip-changes", false, "Skip config file change detection")
	pruneCmd.Flags().BoolVar(&pruneNoFetch, "no-fetch", false, "Skip git fetch --prune (use current remote refs)")
	pruneCmd.Flags().BoolVarP(&pruneDryRun, "dry-run", "n", false, "Show what would be pruned without doing it")
	rootCmd.AddCommand(pruneCmd)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./cmd/wt -run TestPrune`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/wt/prune.go cmd/wt/prune_test.go
git commit -m "feat(prune): add basic prune command with dry-run"
```

---

## Task 7: Implement actual pruning with prompts

**Files:**
- Modify: `cmd/wt/prune.go`
- Test: `cmd/wt/prune_test.go`

**Step 1: Write the failing test**

Add to `cmd/wt/prune_test.go`:

```go
func TestPrune_RemovesGoneBranch(t *testing.T) {
	repoDir, worktreeBase, _ := setupTestRepoWithRemote(t)

	// Create a branch, push it, create worktree, then delete from remote
	cmds := [][]string{
		{"git", "checkout", "-b", "to-prune"},
		{"git", "commit", "--allow-empty", "-m", "prune me"},
		{"git", "push", "-u", "origin", "to-prune"},
		{"git", "checkout", "master"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Create worktree
	mgr := worktree.NewManager(repoDir, worktreeBase)
	_, err := mgr.Create("to-prune", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete branch from remote
	cmd := exec.Command("git", "push", "origin", "--delete", "to-prune")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("push delete failed: %v\n%s", err, out)
	}

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		pruneDryRun = false
		pruneNoFetch = false
		pruneForce = false
		pruneSkipChanges = false
	}()

	// Use --force and --skip-changes to avoid prompts
	rootCmd.SetArgs([]string{"prune",
		"--worktree-base", worktreeBase,
		"--force",
		"--skip-changes",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune failed: %v\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "to-prune") {
		t.Errorf("output should mention pruned branch, got: %s", output)
	}
	if !strings.Contains(output, "Pruned") {
		t.Errorf("output should confirm pruning, got: %s", output)
	}

	// Worktree should be gone
	if mgr.Exists("to-prune") {
		t.Error("worktree should be removed after prune")
	}

	// Local branch should be gone
	if mgr.BranchExists("to-prune") {
		t.Error("local branch should be deleted after prune")
	}
}

func TestPrune_MultipleWorktrees(t *testing.T) {
	repoDir, worktreeBase, _ := setupTestRepoWithRemote(t)

	// Create multiple branches
	branches := []string{"prune-a", "prune-b", "keep-c"}
	for _, branch := range branches {
		cmds := [][]string{
			{"git", "checkout", "-b", branch},
			{"git", "commit", "--allow-empty", "-m", branch + " commit"},
			{"git", "push", "-u", "origin", branch},
			{"git", "checkout", "master"},
		}
		for _, args := range cmds {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = repoDir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v failed: %v\n%s", args, err, out)
			}
		}
	}

	// Create worktrees for all
	mgr := worktree.NewManager(repoDir, worktreeBase)
	for _, branch := range branches {
		_, err := mgr.Create(branch, "")
		if err != nil {
			t.Fatalf("Create %s failed: %v", branch, err)
		}
	}

	// Delete prune-a and prune-b from remote, keep keep-c
	for _, branch := range []string{"prune-a", "prune-b"} {
		cmd := exec.Command("git", "push", "origin", "--delete", branch)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("push delete %s failed: %v\n%s", branch, err, out)
		}
	}

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		pruneDryRun = false
		pruneNoFetch = false
		pruneForce = false
		pruneSkipChanges = false
	}()

	rootCmd.SetArgs([]string{"prune",
		"--worktree-base", worktreeBase,
		"--force",
		"--skip-changes",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune failed: %v\n%s", err, buf.String())
	}

	output := buf.String()

	// Should have pruned a and b
	if !strings.Contains(output, "prune-a") {
		t.Errorf("output should mention prune-a, got: %s", output)
	}
	if !strings.Contains(output, "prune-b") {
		t.Errorf("output should mention prune-b, got: %s", output)
	}

	// prune-a and prune-b worktrees should be gone
	if mgr.Exists("prune-a") {
		t.Error("prune-a worktree should be removed")
	}
	if mgr.Exists("prune-b") {
		t.Error("prune-b worktree should be removed")
	}

	// keep-c should still exist
	if !mgr.Exists("keep-c") {
		t.Error("keep-c worktree should still exist")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./cmd/wt -run TestPrune_Removes -run TestPrune_Multiple`
Expected: FAIL (pruning not implemented yet)

**Step 3: Write implementation**

Update `cmd/wt/prune.go`, replace the `// TODO` section with:

```go
		// Prune each candidate
		var pruned []string
		var errors []string

		configPath := pruneConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		for _, candidate := range candidates {
			branch := candidate.Branch
			wtPath := candidate.Path

			// Check for issues that require prompting
			hasUncommitted := mgr.HasUncommittedChanges(wtPath)
			hasUnpushed := mgr.HasUnpushedCommits(branch)

			if (hasUncommitted || hasUnpushed) && !pruneForce {
				issues := []string{}
				if hasUncommitted {
					issues = append(issues, "uncommitted changes")
				}
				if hasUnpushed {
					issues = append(issues, "unpushed commits")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Remove %s? It has %s [y/n]: ", branch, strings.Join(issues, " and "))

				reader := bufio.NewReader(os.Stdin)
				input, err := reader.ReadString('\n')
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s: failed to read input: %v", branch, err))
					continue
				}
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "y" && input != "yes" {
					fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s\n", branch)
					continue
				}
			}

			// Config file change detection (unless --force or --skip-changes)
			if !pruneForce && !pruneSkipChanges {
				globalCfg, _ := config.LoadGlobalConfig(configPath)
				repoCfg, _ := config.LoadRepoConfig(repoRoot)
				cfg := config.MergeConfigs(globalCfg, repoCfg)

				if len(cfg.CopyFiles) > 0 {
					changes, err := mgr.DetectChanges(wtPath, cfg.CopyFiles)
					if err != nil {
						errors = append(errors, fmt.Sprintf("%s: detecting changes: %v", branch, err))
						continue
					}

					if len(changes) > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "\n%s has modified config files:\n", branch)
						for _, c := range changes {
							conflict := ""
							if c.Conflict {
								conflict = " (CONFLICT: source also changed)"
							}
							fmt.Fprintf(cmd.OutOrStdout(), "  %s%s\n", c.File, conflict)
						}
						fmt.Fprintln(cmd.OutOrStdout())
						fmt.Fprintln(cmd.OutOrStdout(), "[m] Merge back to main worktree")
						fmt.Fprintln(cmd.OutOrStdout(), "[k] Keep original (discard changes)")
						fmt.Fprintln(cmd.OutOrStdout(), "[s] Skip this worktree")
						fmt.Fprintln(cmd.OutOrStdout(), "[a] Abort prune")
						fmt.Fprint(cmd.OutOrStdout(), "Choice: ")

						reader := bufio.NewReader(os.Stdin)
						input, err := reader.ReadString('\n')
						if err != nil {
							errors = append(errors, fmt.Sprintf("%s: reading input: %v", branch, err))
							continue
						}
						input = strings.TrimSpace(strings.ToLower(input))

						switch input {
						case "m":
							for _, c := range changes {
								if c.Conflict {
									fmt.Fprintf(cmd.ErrOrStderr(), "Skipping %s due to conflict\n", c.File)
									continue
								}
								if err := mgr.MergeBack(wtPath, c.File); err != nil {
									fmt.Fprintf(cmd.ErrOrStderr(), "Failed to merge %s: %v\n", c.File, err)
								} else {
									fmt.Fprintf(cmd.OutOrStdout(), "Merged %s\n", c.File)
								}
							}
						case "k":
							// Continue with removal
						case "s":
							fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s\n", branch)
							continue
						case "a":
							// Report what was already pruned before aborting
							if len(pruned) > 0 {
								fmt.Fprintf(cmd.OutOrStdout(), "\nPruned %d worktree(s) before abort:\n", len(pruned))
								for _, p := range pruned {
									fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
								}
							}
							return fmt.Errorf("aborted")
						default:
							errors = append(errors, fmt.Sprintf("%s: invalid choice", branch))
							continue
						}
					}
				}
			}

			// Remove worktree
			if err := mgr.Remove(branch, pruneForce); err != nil {
				errors = append(errors, fmt.Sprintf("%s: remove worktree: %v", branch, err))
				continue
			}

			// Delete local branch
			if err := mgr.DeleteBranch(branch); err != nil {
				// Worktree is already gone, just warn
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: removed worktree but failed to delete branch %s: %v\n", branch, err)
			}

			pruned = append(pruned, branch)
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
```

Also add these imports at the top of the file:

```go
import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./cmd/wt -run TestPrune`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/wt/prune.go cmd/wt/prune_test.go
git commit -m "feat(prune): implement actual pruning with prompts"
```

---

## Task 8: Run full test suite and verify

**Step 1: Run all tests**

Run: `go test -v ./...`
Expected: All tests pass

**Step 2: Manual smoke test**

```bash
# Build
go build -o wt-bin ./cmd/wt

# Test help
./wt-bin prune --help

# Test in a repo (dry-run first)
./wt-bin prune --dry-run
```

**Step 3: Commit any fixes if needed**

If tests fail, fix and commit with appropriate message.

---

## Task 9: Update design doc with any changes

**Files:**
- Modify: `docs/plans/2026-02-04-prune-design.md` (if implementation deviated)

Review the implementation against the design. If there were any changes, update the design doc to reflect actual implementation.

**Commit if changes made:**

```bash
git add docs/plans/2026-02-04-prune-design.md
git commit -m "docs: update prune design to match implementation"
```

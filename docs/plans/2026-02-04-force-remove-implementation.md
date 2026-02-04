# Force Remove Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `wt remove --force` actually force git to remove dirty worktrees, and rename the prompt-skip behavior to `--skip-changes`.

**Architecture:** Add `force` parameter to `Manager.Remove()` that passes `--force` to git. Update CLI flags so `-f/--force` controls git forcing and implies `--skip-changes`.

**Tech Stack:** Go, Cobra CLI, git worktree

---

### Task 1: Add force parameter to Manager.Remove()

**Files:**
- Modify: `internal/worktree/worktree.go:169-183`
- Test: `internal/worktree/worktree_test.go`

**Step 1: Write the failing test for force=true**

Add to `internal/worktree/worktree_test.go`:

```go
func TestManager_Remove_Force(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	// Create worktree
	wtPath, err := mgr.Create("dirty-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Make worktree dirty (uncommitted changes)
	testFile := filepath.Join(wtPath, "dirty.txt")
	if err := os.WriteFile(testFile, []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	// Force remove should succeed
	err = mgr.Remove("dirty-branch", true)
	if err != nil {
		t.Errorf("Remove with force=true should succeed on dirty worktree: %v", err)
	}

	// Verify worktree is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree should be removed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worktree -run TestManager_Remove_Force`
Expected: Compilation error - `Remove` takes 1 argument, not 2

**Step 3: Write the failing test for force=false**

Add to `internal/worktree/worktree_test.go`:

```go
func TestManager_Remove_NoForce_DirtyFails(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	// Create worktree
	wtPath, err := mgr.Create("dirty-branch-2", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Make worktree dirty (uncommitted changes)
	testFile := filepath.Join(wtPath, "dirty.txt")
	if err := os.WriteFile(testFile, []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	// Non-force remove should fail on dirty worktree
	err = mgr.Remove("dirty-branch-2", false)
	if err == nil {
		t.Error("Remove with force=false should fail on dirty worktree")
	}
}
```

**Step 4: Update Manager.Remove() signature and implementation**

In `internal/worktree/worktree.go`, replace lines 169-183:

```go
// Remove removes a worktree by branch name.
// If force is true, removes even if worktree has uncommitted changes.
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

**Step 5: Run tests to verify they pass**

Run: `go test -v ./internal/worktree -run TestManager_Remove`
Expected: Compilation errors - callers need updating

**Step 6: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "feat(worktree): add force parameter to Remove method"
```

---

### Task 2: Update remove command flags

**Files:**
- Modify: `cmd/wt/remove.go:14-18,56-57,128`

**Step 1: Update flag variables**

In `cmd/wt/remove.go`, replace lines 14-18:

```go
var (
	removeWorktreeBase string
	removeConfigPath   string
	removeForce        bool
	removeSkipChanges  bool
)
```

**Step 2: Update the condition for skipping prompts**

In `cmd/wt/remove.go`, replace line 56-57:

```go
		// Check for config file changes (unless --force or --skip-changes)
		if !removeForce && !removeSkipChanges {
```

**Step 3: Update Manager.Remove call**

In `cmd/wt/remove.go`, replace line 116:

```go
		if err := mgr.Remove(branch, removeForce); err != nil {
```

**Step 4: Update flag registration**

In `cmd/wt/remove.go`, replace line 128:

```go
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Force removal even if worktree has uncommitted changes")
	removeCmd.Flags().BoolVar(&removeSkipChanges, "skip-changes", false, "Skip config file change detection")
```

**Step 5: Run existing tests to verify no regressions**

Run: `go test -v ./cmd/wt -run TestRemoveCommand`
Expected: PASS (existing test uses `--force` which now also forces git)

**Step 6: Commit**

```bash
git add cmd/wt/remove.go
git commit -m "feat(remove): consolidate --force flag to force git removal

--force now forces git removal AND skips config prompts.
--skip-changes preserves old prompt-skip-only behavior."
```

---

### Task 3: Add integration tests for force behavior

**Files:**
- Modify: `cmd/wt/new_test.go`

**Step 1: Write test for --force removing dirty worktree**

Add to `cmd/wt/new_test.go`:

```go
func TestRemoveForce_DirtyWorktree(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create a worktree
	mgr := worktree.NewManager(repoDir, worktreeBase)
	wtPath, _ := mgr.Create("dirty-feature", "")

	// Make it dirty with uncommitted changes
	testFile := filepath.Join(wtPath, "dirty.txt")
	if err := os.WriteFile(testFile, []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	// Remove with --force should succeed
	rootCmd.SetArgs([]string{"remove", "dirty-feature",
		"--worktree-base", worktreeBase,
		"--force",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("remove --force should succeed on dirty worktree: %v\n%s", err, buf.String())
	}

	// Verify worktree is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree should be removed")
	}
}
```

**Step 2: Run the test**

Run: `go test -v ./cmd/wt -run TestRemoveForce_DirtyWorktree`
Expected: PASS

**Step 3: Write test for --skip-changes NOT removing dirty worktree**

Add to `cmd/wt/new_test.go`:

```go
func TestRemoveSkipChanges_DirtyWorktreeFails(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create a worktree
	mgr := worktree.NewManager(repoDir, worktreeBase)
	wtPath, _ := mgr.Create("skip-dirty", "")

	// Make it dirty with uncommitted changes
	testFile := filepath.Join(wtPath, "dirty.txt")
	if err := os.WriteFile(testFile, []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	// Remove with --skip-changes alone should fail on dirty worktree
	rootCmd.SetArgs([]string{"remove", "skip-dirty",
		"--worktree-base", worktreeBase,
		"--skip-changes",
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("remove --skip-changes should fail on dirty worktree (git blocks it)")
	}

	// Verify worktree still exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree should NOT be removed")
	}
}
```

**Step 4: Run the test**

Run: `go test -v ./cmd/wt -run TestRemoveSkipChanges_DirtyWorktreeFails`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/wt/new_test.go
git commit -m "test(remove): add integration tests for --force and --skip-changes"
```

---

### Task 4: Run full test suite and verify

**Step 1: Run all tests**

Run: `go test -v ./...`
Expected: All tests PASS

**Step 2: Manual verification**

Create a dirty worktree and test both flags:

```bash
# Build
go build -o wt-bin ./cmd/wt

# Create worktree
./wt-bin new test-dirty

# Make it dirty
cd ~/.local/share/wt/worktrees/*/test-dirty
echo "dirty" > dirty.txt
cd -

# This should fail
./wt-bin remove test-dirty
# Expected: error about uncommitted changes

# This should succeed
./wt-bin remove test-dirty --force
# Expected: success
```

**Step 3: Commit if any fixes needed**

If fixes were needed, commit them with appropriate message.

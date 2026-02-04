# Base Branch Flag Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `-b/--base <branch>` flag to `wt new` to specify which branch to base the new branch on.

**Architecture:** The Manager gains a `FetchBranch` method for fetching from origin. The `Create` method signature changes to accept an optional base branch. The CLI validates that `-b` can only be used when creating a new branch.

**Tech Stack:** Go, Cobra CLI, git subprocess calls

---

### Task 1: Add FetchBranch method to Manager

**Files:**
- Modify: `internal/worktree/worktree.go:58` (after RemoteBranchExists)
- Test: `internal/worktree/worktree_test.go`

**Step 1: Write the failing test**

Add to `internal/worktree/worktree_test.go`:

```go
func TestFetchBranch(t *testing.T) {
	mainRepo, bareRemote, worktreeBase := setupRepoWithRemote(t)

	// Create a branch in a separate clone and push it (without fetching in mainRepo)
	tmpClone := filepath.Join(t.TempDir(), "tmpclone")
	cmd := exec.Command("git", "clone", bareRemote, tmpClone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}

	cmds := [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "unfetched-branch"},
		{"git", "commit", "--allow-empty", "-m", "unfetched commit"},
		{"git", "push", "-u", "origin", "unfetched-branch"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpClone
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	mgr := NewManager(mainRepo, worktreeBase)

	// Branch should not be known locally yet
	if mgr.RemoteBranchExists("unfetched-branch") {
		t.Fatal("branch should not be known before fetch")
	}

	// Fetch the branch
	err := mgr.FetchBranch("unfetched-branch")
	if err != nil {
		t.Fatalf("FetchBranch failed: %v", err)
	}

	// Now it should be known
	if !mgr.RemoteBranchExists("unfetched-branch") {
		t.Error("branch should exist after fetch")
	}
}

func TestFetchBranchNotFound(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	err := mgr.FetchBranch("nonexistent-branch")
	if err == nil {
		t.Fatal("expected error for nonexistent branch")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worktree -run TestFetchBranch`
Expected: FAIL with "mgr.FetchBranch undefined"

**Step 3: Write minimal implementation**

Add to `internal/worktree/worktree.go` after line 58:

```go
// FetchBranch fetches a specific branch from origin.
// Returns an error if the branch doesn't exist on the remote.
func (m *Manager) FetchBranch(branch string) error {
	cmd := exec.Command("git", "fetch", "origin", branch)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch origin %s: %w: %s", branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/worktree -run TestFetchBranch`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "feat(worktree): add FetchBranch method"
```

---

### Task 2: Add ErrBaseBranchNotFound sentinel error

**Files:**
- Modify: `internal/worktree/worktree.go:14-16` (error declarations)

**Step 1: Add the error**

Add to the error declarations at top of `internal/worktree/worktree.go`:

```go
var ErrBaseBranchNotFound = errors.New("base branch not found")
```

**Step 2: Verify it compiles**

Run: `go build ./internal/worktree`
Expected: Success

**Step 3: Commit**

```bash
git add internal/worktree/worktree.go
git commit -m "feat(worktree): add ErrBaseBranchNotFound error"
```

---

### Task 3: Modify Create to accept baseBranch parameter

**Files:**
- Modify: `internal/worktree/worktree.go:64-98` (Create method)
- Modify: `cmd/wt/new.go:62` (caller)
- Test: `internal/worktree/worktree_test.go`

**Step 3.1: Write the failing test for Create with base branch**

Add to `internal/worktree/worktree_test.go`:

```go
func TestCreateWithBaseBranch(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)

	// Create a develop branch locally
	cmd := exec.Command("git", "checkout", "-b", "develop")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create develop failed: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "develop commit")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit failed: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "checkout", "master")
	cmd.Dir = mainRepo
	cmd.CombinedOutput() // ignore error, might be main

	mgr := NewManager(mainRepo, worktreeBase)

	// Create feature branch based on develop
	wtPath, err := mgr.Create("feature-from-develop", "develop")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify worktree is based on develop (check parent commit message)
	cmd = exec.Command("git", "log", "-1", "--format=%s", "HEAD")
	cmd.Dir = wtPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log failed: %v\n%s", err, out)
	}

	if strings.TrimSpace(string(out)) != "develop commit" {
		t.Errorf("expected branch to be based on develop, got parent: %s", out)
	}
}

func TestCreateWithBaseBranchNotFound(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	_, err := mgr.Create("feature-x", "nonexistent-base")
	if err == nil {
		t.Fatal("expected error for nonexistent base branch")
	}
	if !errors.Is(err, ErrBaseBranchNotFound) {
		t.Errorf("expected ErrBaseBranchNotFound, got: %v", err)
	}
}

func TestCreateWithEmptyBaseBranch(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	// Empty base branch should use current behavior (base on HEAD)
	wtPath, err := mgr.Create("feature-default", "")
	if err != nil {
		t.Fatalf("Create with empty base failed: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree should exist")
	}
}
```

**Step 3.2: Run test to verify it fails**

Run: `go test -v ./internal/worktree -run TestCreateWith`
Expected: FAIL - too many arguments to Create

**Step 3.3: Update Create method signature and implementation**

Replace the `Create` method in `internal/worktree/worktree.go`:

```go
// Create creates a new worktree for the given branch.
// If baseBranch is specified, creates a new branch based on it.
// If baseBranch is empty:
//   - If the branch exists locally, uses it directly.
//   - If the branch exists only on origin, creates a local tracking branch.
//   - If the branch doesn't exist anywhere, creates a new branch from HEAD.
func (m *Manager) Create(branch, baseBranch string) (string, error) {
	wtPath := m.WorktreePath(branch)

	if m.Exists(branch) {
		return "", ErrWorktreeExists
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", err
	}

	// If base branch specified, resolve it and create new branch based on it
	if baseBranch != "" {
		baseRef := baseBranch
		if !m.BranchExists(baseBranch) {
			// Try to fetch from origin
			if err := m.FetchBranch(baseBranch); err != nil {
				return "", ErrBaseBranchNotFound
			}
			if !m.RemoteBranchExists(baseBranch) {
				return "", ErrBaseBranchNotFound
			}
			baseRef = "origin/" + baseBranch
		}
		cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, baseRef)
		cmd.Dir = m.RepoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return wtPath, nil
	}

	// No base branch - use existing behavior
	localExists := m.BranchExists(branch)
	remoteExists := m.RemoteBranchExists(branch)

	var cmd *exec.Cmd
	switch {
	case localExists:
		// Local branch exists - use it directly
		cmd = exec.Command("git", "worktree", "add", wtPath, branch)
	case remoteExists:
		// Remote branch exists - create local tracking branch
		cmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath, "origin/"+branch)
	default:
		// No branch exists - create new branch
		cmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	}
	cmd.Dir = m.RepoRoot

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return wtPath, nil
}
```

**Step 3.4: Run worktree tests**

Run: `go test -v ./internal/worktree -run TestCreate`
Expected: PASS

**Step 3.5: Update callers**

The `Create` method is called in several places. Update all callers to pass empty string for baseBranch.

In `cmd/wt/new.go` line 62, change:
```go
wtPath, err := mgr.Create(branch)
```
to:
```go
wtPath, err := mgr.Create(branch, "")
```

Search for other callers and update them similarly. Check:
- `cmd/wt/switch.go` - likely calls Create
- `cmd/wt/new_test.go` - test setup may call Create directly

**Step 3.6: Run all tests**

Run: `go test -v ./...`
Expected: PASS

**Step 3.7: Commit**

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go cmd/wt/new.go cmd/wt/switch.go
git commit -m "feat(worktree): add baseBranch parameter to Create"
```

---

### Task 4: Add -b/--base flag to wt new command

**Files:**
- Modify: `cmd/wt/new.go`
- Test: `cmd/wt/new_test.go`

**Step 4.1: Write the failing test**

Add to `cmd/wt/new_test.go`:

```go
func TestNewCommandWithBaseBranch(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	// Create a develop branch
	cmd := exec.Command("git", "checkout", "-b", "develop")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create develop failed: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "develop commit")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit failed: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "checkout", "master")
	cmd.Dir = repoDir
	cmd.CombinedOutput() // ignore error

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
	}()

	rootCmd.SetArgs([]string{"new", "feature-from-base", "-b", "develop",
		"--worktree-base", worktreeBase,
		"--print-path",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("new command failed: %v\n%s", err, buf.String())
	}

	expectedPath := filepath.Join(worktreeBase, "myrepo", "feature-from-base")
	output := strings.TrimSpace(buf.String())
	if output != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, output)
	}

	// Verify branch is based on develop
	cmd = exec.Command("git", "log", "-1", "--format=%s", "HEAD")
	cmd.Dir = expectedPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "develop commit" {
		t.Errorf("expected based on develop, got: %s", out)
	}
}
```

**Step 4.2: Run test to verify it fails**

Run: `go test -v ./cmd/wt -run TestNewCommandWithBaseBranch`
Expected: FAIL - unknown flag -b

**Step 4.3: Add the flag and wire it up**

In `cmd/wt/new.go`, add to the var block:

```go
var (
	newPrintPath    bool
	newWorktreeBase string
	newConfigPath   string
	newBaseBranch   string
)
```

In the `init()` function, add:

```go
newCmd.Flags().StringVarP(&newBaseBranch, "base", "b", "", "Base branch for the new branch")
```

In the `RunE` function, change the Create call:

```go
wtPath, err := mgr.Create(branch, newBaseBranch)
```

**Step 4.4: Run test to verify it passes**

Run: `go test -v ./cmd/wt -run TestNewCommandWithBaseBranch`
Expected: PASS

**Step 4.5: Commit**

```bash
git add cmd/wt/new.go cmd/wt/new_test.go
git commit -m "feat(new): add -b/--base flag to specify base branch"
```

---

### Task 5: Add validation for -b with existing branch

**Files:**
- Modify: `cmd/wt/new.go`
- Test: `cmd/wt/new_test.go`

**Step 5.1: Write the failing test**

Add to `cmd/wt/new_test.go`:

```go
func TestNewCommandBaseBranchWithExistingBranch(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	// Create a branch that already exists
	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create branch failed: %v\n%s", err, out)
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
	}()

	// Try to create worktree with -b for an existing branch
	rootCmd.SetArgs([]string{"new", "existing-branch", "-b", "master",
		"--worktree-base", worktreeBase,
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when using -b with existing branch")
	}
	if !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "--base") {
		t.Errorf("error should mention branch exists and --base cannot be applied, got: %v", err)
	}
}
```

**Step 5.2: Run test to verify it fails**

Run: `go test -v ./cmd/wt -run TestNewCommandBaseBranchWithExistingBranch`
Expected: FAIL - test expects specific error message

**Step 5.3: Add validation in new.go**

In `cmd/wt/new.go`, add validation before the Create call:

```go
// Validate -b flag usage
if newBaseBranch != "" {
	if mgr.BranchExists(branch) || mgr.RemoteBranchExists(branch) {
		return fmt.Errorf("branch '%s' already exists, cannot apply --base", branch)
	}
}
```

**Step 5.4: Run test to verify it passes**

Run: `go test -v ./cmd/wt -run TestNewCommandBaseBranchWithExistingBranch`
Expected: PASS

**Step 5.5: Commit**

```bash
git add cmd/wt/new.go cmd/wt/new_test.go
git commit -m "feat(new): error when -b used with existing branch"
```

---

### Task 6: Update README documentation

**Files:**
- Modify: `README.md:30-37`

**Step 6.1: Update the documentation**

Replace the "Create a new worktree" section in `README.md`:

```markdown
### Create a new worktree

```bash
wt new feature-branch
# Creates worktree at ~/.local/share/wt/worktrees/<repo>/feature-branch
# Copies configured files from main worktree
# cd's into the new worktree (via shell function)

wt new feature-branch -b develop
# Creates feature-branch based on develop instead of HEAD
# Fetches from origin if develop isn't available locally
```
```

**Step 6.2: Verify documentation renders correctly**

Run: `cat README.md | head -50`
Expected: Updated documentation visible

**Step 6.3: Commit**

```bash
git add README.md
git commit -m "docs: document -b flag for wt new"
```

---

### Task 7: Run full test suite and verify

**Step 7.1: Run all tests**

Run: `go test -v ./...`
Expected: All tests pass

**Step 7.2: Manual smoke test**

```bash
# Build
go build -o wt-bin ./cmd/wt

# Test help shows new flag
./wt-bin new --help

# Create a test branch based on another
./wt-bin new test-feature -b main --print-path
```

**Step 7.3: Final commit if any cleanup needed**

If any issues found, fix and commit with appropriate message.

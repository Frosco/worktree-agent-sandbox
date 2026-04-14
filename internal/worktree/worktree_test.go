package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupRepoWithRemote creates a main repo with a bare remote, returns paths to both
func setupRepoWithRemote(t *testing.T) (mainRepo, bareRemote string) {
	t.Helper()
	tmpDir := t.TempDir()
	bareRemote = filepath.Join(tmpDir, "remote.git")
	mainRepo = filepath.Join(tmpDir, "local")

	// Create bare remote
	if err := os.MkdirAll(bareRemote, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = bareRemote
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}

	// Clone the bare remote to create local repo
	cmd = exec.Command("git", "clone", bareRemote, mainRepo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}

	// Configure user for commits
	cmds := [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
		{"git", "push", "-u", "origin", "HEAD"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = mainRepo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	return mainRepo, bareRemote
}

// createWorktreeInRepo creates a git worktree at .claude/worktrees/<name> inside mainRepo.
func createWorktreeInRepo(t *testing.T, mainRepo, name, branch string) string {
	t.Helper()
	wtPath := filepath.Join(mainRepo, ".claude", "worktrees", name)
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v\n%s", err, out)
	}
	return wtPath
}

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

func TestRemoteBranchExists(t *testing.T) {
	mainRepo, bareRemote := setupRepoWithRemote(t)

	// Create a branch in a separate clone and push it
	tmpClone := filepath.Join(t.TempDir(), "tmpclone")
	cmd := exec.Command("git", "clone", bareRemote, tmpClone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}

	cmds := [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "remote-only-branch"},
		{"git", "commit", "--allow-empty", "-m", "remote commit"},
		{"git", "push", "-u", "origin", "remote-only-branch"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpClone
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Fetch in main repo so it knows about the remote branch
	cmd = exec.Command("git", "fetch", "origin")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fetch failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo)

	// Local branch should not exist
	if mgr.BranchExists("remote-only-branch") {
		t.Error("local branch should not exist")
	}

	// Remote branch should exist
	if !mgr.RemoteBranchExists("remote-only-branch") {
		t.Error("remote branch should exist")
	}

	// Non-existent branch should not exist
	if mgr.RemoteBranchExists("nonexistent-branch") {
		t.Error("nonexistent branch should not be found")
	}
}

func TestManager_Remove_Force(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Create worktree at .claude/worktrees/dirty-branch
	wtPath := createWorktreeInRepo(t, mainRepo, "dirty-branch", "dirty-branch")

	// Make worktree dirty (uncommitted changes)
	testFile := filepath.Join(wtPath, "dirty.txt")
	if err := os.WriteFile(testFile, []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	// Force remove should succeed
	err := mgr.Remove("dirty-branch", true)
	if err != nil {
		t.Errorf("Remove with force=true should succeed on dirty worktree: %v", err)
	}

	// Verify worktree is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree should be removed")
	}

	// Verify branch is also deleted
	if mgr.BranchExists("dirty-branch") {
		t.Error("branch should be deleted after Remove")
	}
}

func TestManager_Remove_NoForce_DirtyFails(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Create worktree at .claude/worktrees/dirty-branch-2
	wtPath := createWorktreeInRepo(t, mainRepo, "dirty-branch-2", "dirty-branch-2")

	// Make worktree dirty (uncommitted changes)
	testFile := filepath.Join(wtPath, "dirty.txt")
	if err := os.WriteFile(testFile, []byte("uncommitted"), 0644); err != nil {
		t.Fatal(err)
	}

	// Non-force remove should fail on dirty worktree
	err := mgr.Remove("dirty-branch-2", false)
	if err == nil {
		t.Error("Remove with force=false should fail on dirty worktree")
	}
}

func TestManager_Remove_DeletesBranch(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Create worktree
	createWorktreeInRepo(t, mainRepo, "to-remove", "branch-to-remove")

	// Verify branch exists before removal
	if !mgr.BranchExists("branch-to-remove") {
		t.Fatal("branch should exist before Remove")
	}

	// Remove the worktree
	err := mgr.Remove("to-remove", false)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Branch should be deleted
	if mgr.BranchExists("branch-to-remove") {
		t.Error("branch should be deleted after Remove")
	}
}

func TestBranchUpstream_WithTracking(t *testing.T) {
	mainRepo, bareRemote := setupRepoWithRemote(t)

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

	mgr := NewManager(mainRepo)

	upstream := mgr.BranchUpstream("tracked-branch")
	if upstream != "origin/tracked-branch" {
		t.Errorf("expected 'origin/tracked-branch', got %q", upstream)
	}
}

func TestBranchUpstream_NoTracking(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)

	// Create a local-only branch (no push, no tracking)
	cmd := exec.Command("git", "checkout", "-b", "local-only")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo)

	upstream := mgr.BranchUpstream("local-only")
	if upstream != "" {
		t.Errorf("expected empty string for local-only branch, got %q", upstream)
	}
}

func TestDeleteBranch(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)

	// Create a branch
	cmd := exec.Command("git", "branch", "branch-to-delete")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create branch failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo)

	// Verify branch exists
	if !mgr.BranchExists("branch-to-delete") {
		t.Fatal("branch should exist before delete")
	}

	// Delete the branch (not force - branch has no commits ahead of master)
	err := mgr.DeleteBranch("branch-to-delete", false)
	if err != nil {
		t.Fatalf("DeleteBranch failed: %v", err)
	}

	// Verify branch is gone
	if mgr.BranchExists("branch-to-delete") {
		t.Error("branch should not exist after delete")
	}
}

func TestDeleteBranch_NotFound(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	err := mgr.DeleteBranch("nonexistent-branch", false)
	if err == nil {
		t.Error("expected error for nonexistent branch")
	}
}

func TestHasUncommittedChanges_Clean(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Create a clean worktree
	wtPath := createWorktreeInRepo(t, mainRepo, "clean-branch", "clean-branch")

	if mgr.HasUncommittedChanges(wtPath) {
		t.Error("clean worktree should not have uncommitted changes")
	}
}

func TestHasUncommittedChanges_Modified(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Create worktree
	wtPath := createWorktreeInRepo(t, mainRepo, "modified-branch", "modified-branch")

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
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Create worktree
	wtPath := createWorktreeInRepo(t, mainRepo, "staged-branch", "staged-branch")

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

func TestHasUnpushedCommits_NoneAhead(t *testing.T) {
	mainRepo, bareRemote := setupRepoWithRemote(t)

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

	mgr := NewManager(mainRepo)

	if mgr.HasUnpushedCommits("synced-branch") {
		t.Error("fully synced branch should not have unpushed commits")
	}
}

func TestHasUnpushedCommits_Ahead(t *testing.T) {
	mainRepo, bareRemote := setupRepoWithRemote(t)

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

	mgr := NewManager(mainRepo)

	if !mgr.HasUnpushedCommits("ahead-branch") {
		t.Error("branch with local commit should have unpushed commits")
	}
}

func TestHasUnpushedCommits_NoUpstream(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)

	// Create local-only branch (no upstream)
	cmd := exec.Command("git", "checkout", "-b", "no-upstream")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo)

	// No upstream means we can't determine - treat as "no unpushed" for prune safety
	// (branches without upstream won't be pruned anyway)
	if mgr.HasUnpushedCommits("no-upstream") {
		t.Error("branch with no upstream should return false")
	}
}

func TestFetchPrune(t *testing.T) {
	mainRepo, bareRemote := setupRepoWithRemote(t)

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

	mgr := NewManager(mainRepo)

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

func TestList(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Create two worktrees
	createWorktreeInRepo(t, mainRepo, "feature-a", "feature-a")
	createWorktreeInRepo(t, mainRepo, "feature-b", "feature-b")

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(list))
	}

	// Collect names for easier checking (order may vary)
	names := make(map[string]bool)
	for _, wt := range list {
		names[wt.Name] = true
		// Verify branch is populated
		if wt.Branch == "" {
			t.Errorf("worktree %q has empty branch", wt.Name)
		}
		// Verify path is populated and correct
		expectedPath := filepath.Join(mainRepo, ".claude", "worktrees", wt.Name)
		if wt.Path != expectedPath {
			t.Errorf("worktree %q path = %q, want %q", wt.Name, wt.Path, expectedPath)
		}
	}

	if !names["feature-a"] || !names["feature-b"] {
		t.Errorf("expected feature-a and feature-b, got %v", names)
	}
}

func TestList_Empty(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// No worktrees created, .claude/worktrees/ doesn't exist
	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if list != nil {
		t.Errorf("expected nil list, got %v", list)
	}
}

func TestList_BranchFromGit(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Create a worktree with a specific branch name
	createWorktreeInRepo(t, mainRepo, "my-feature", "wt-my-feature")

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(list) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(list))
	}

	if list[0].Name != "my-feature" {
		t.Errorf("Name = %q, want %q", list[0].Name, "my-feature")
	}
	if list[0].Branch != "wt-my-feature" {
		t.Errorf("Branch = %q, want %q", list[0].Branch, "wt-my-feature")
	}
}

func TestExists(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Worktree doesn't exist
	if mgr.Exists("nonexistent") {
		t.Error("Exists should return false for nonexistent worktree")
	}

	// Create worktree
	createWorktreeInRepo(t, mainRepo, "exists-test", "exists-test")

	// Now it exists
	if !mgr.Exists("exists-test") {
		t.Error("Exists should return true for existing worktree")
	}
}

func TestRemove_NotFound(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	err := mgr.Remove("nonexistent", false)
	if err != ErrWorktreeNotFound {
		t.Errorf("expected ErrWorktreeNotFound, got: %v", err)
	}
}

func TestList_IgnoresFiles(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	// Create the worktrees directory with a regular file in it
	wtDir := filepath.Join(mainRepo, ".claude", "worktrees")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, "not-a-worktree.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create one real worktree
	createWorktreeInRepo(t, mainRepo, "real-wt", "real-wt")

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should only have the directory, not the file
	if len(list) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(list))
	}
	if list[0].Name != "real-wt" {
		t.Errorf("Name = %q, want %q", list[0].Name, "real-wt")
	}
}

func TestBranchForWorktree(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)

	wtPath := createWorktreeInRepo(t, mainRepo, "branch-check", "wt-branch-check")

	// Verify branch detection via List
	branch := branchForWorktree(wtPath)
	if branch != "wt-branch-check" {
		t.Errorf("branchForWorktree = %q, want %q", branch, "wt-branch-check")
	}
}

func TestBranchForWorktree_InvalidPath(t *testing.T) {
	branch := branchForWorktree("/nonexistent/path")
	if branch != "" {
		t.Errorf("expected empty string for invalid path, got %q", branch)
	}
}

func TestRemove_CleanWorktree(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo)

	wtPath := createWorktreeInRepo(t, mainRepo, "clean-remove", "clean-remove")

	err := mgr.Remove("clean-remove", false)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify worktree directory is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}

	// Verify branch is deleted
	if mgr.BranchExists("clean-remove") {
		t.Error("branch should be deleted after Remove")
	}
}

func TestCreate(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)

	// Push a branch to the remote that doesn't exist locally
	cmd := exec.Command("git", "checkout", "-b", "feature-review")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(mainRepo, "feature.txt"), []byte("feature content"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"add", "feature.txt"},
		{"commit", "-m", "add feature"},
		{"push", "origin", "feature-review"},
		{"checkout", "main"},
		{"branch", "-D", "feature-review"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = mainRepo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	mgr := NewManager(mainRepo)
	err := mgr.Create("feature-review", "origin/feature-review")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Verify worktree directory exists
	wtPath := mgr.WorktreePath("feature-review")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree directory not created at %s", wtPath)
	}

	// Verify the local branch is checked out
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = wtPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %v\n%s", err, out)
	}
	if branch := strings.TrimSpace(string(out)); branch != "feature-review" {
		t.Errorf("expected branch feature-review, got %q", branch)
	}

	// Verify the file from the remote branch is present
	content, err := os.ReadFile(filepath.Join(wtPath, "feature.txt"))
	if err != nil {
		t.Fatalf("feature.txt not found: %v", err)
	}
	if string(content) != "feature content" {
		t.Errorf("unexpected content: %q", string(content))
	}
}

func TestCreate_BranchAlreadyExists(t *testing.T) {
	mainRepo, _ := setupRepoWithRemote(t)

	// "main" branch already exists locally — Create should fail
	mgr := NewManager(mainRepo)
	err := mgr.Create("main", "origin/main")
	if err == nil {
		t.Fatal("expected error when branch already exists locally")
	}
}

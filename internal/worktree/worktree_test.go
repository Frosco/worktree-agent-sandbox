package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupRepoWithRemote creates a main repo with a bare remote, returns paths to both
func setupRepoWithRemote(t *testing.T) (mainRepo, bareRemote, worktreeBase string) {
	t.Helper()
	tmpDir := t.TempDir()
	bareRemote = filepath.Join(tmpDir, "remote.git")
	mainRepo = filepath.Join(tmpDir, "local")
	worktreeBase = filepath.Join(tmpDir, "worktrees")

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

	return mainRepo, bareRemote, worktreeBase
}

func TestRemoteBranchExists(t *testing.T) {
	mainRepo, bareRemote, worktreeBase := setupRepoWithRemote(t)

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

	mgr := NewManager(mainRepo, worktreeBase)

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

func TestCreateWorktreeForRemoteBranch(t *testing.T) {
	mainRepo, bareRemote, worktreeBase := setupRepoWithRemote(t)

	// Create a branch in a separate clone and push it
	tmpClone := filepath.Join(t.TempDir(), "tmpclone")
	cmd := exec.Command("git", "clone", bareRemote, tmpClone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}

	cmds := [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "feature-from-remote"},
		{"git", "commit", "--allow-empty", "-m", "feature commit"},
		{"git", "push", "-u", "origin", "feature-from-remote"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpClone
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Fetch in main repo
	cmd = exec.Command("git", "fetch", "origin")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fetch failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo, worktreeBase)

	// Create worktree for the remote branch
	wtPath, err := mgr.Create("feature-from-remote")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	expectedPath := filepath.Join(worktreeBase, "local", "feature-from-remote")
	if wtPath != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, wtPath)
	}

	// Verify worktree exists and is on the correct branch
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = wtPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch failed: %v\n%s", err, out)
	}

	branch := string(out)
	if branch != "feature-from-remote\n" {
		t.Errorf("expected branch 'feature-from-remote', got %q", branch)
	}

	// Verify tracking is set up
	cmd = exec.Command("git", "config", "branch.feature-from-remote.remote")
	cmd.Dir = wtPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git config failed: %v\n%s", err, out)
	}

	remote := string(out)
	if remote != "origin\n" {
		t.Errorf("expected remote 'origin', got %q", remote)
	}
}

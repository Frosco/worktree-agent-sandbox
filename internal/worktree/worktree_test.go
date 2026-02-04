package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	wtPath, err := mgr.Create("feature-from-remote", "")
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

func TestCreateWithRemoteBaseBranch(t *testing.T) {
	mainRepo, bareRemote, worktreeBase := setupRepoWithRemote(t)

	// Create a branch in a separate clone and push it (not fetched in mainRepo)
	tmpClone := filepath.Join(t.TempDir(), "tmpclone")
	cmd := exec.Command("git", "clone", bareRemote, tmpClone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}

	cmds := [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "remote-base"},
		{"git", "commit", "--allow-empty", "-m", "remote base commit"},
		{"git", "push", "-u", "origin", "remote-base"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpClone
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	mgr := NewManager(mainRepo, worktreeBase)

	// Verify base branch is NOT known locally yet
	if mgr.BranchExists("remote-base") || mgr.RemoteBranchExists("remote-base") {
		t.Fatal("base branch should not be known before Create")
	}

	// Create feature branch based on remote-only base branch
	wtPath, err := mgr.Create("feature-from-remote-base", "remote-base")
	if err != nil {
		t.Fatalf("Create with remote base failed: %v", err)
	}

	// Verify worktree is based on remote-base (check commit message)
	cmd = exec.Command("git", "log", "-1", "--format=%s", "HEAD")
	cmd.Dir = wtPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log failed: %v\n%s", err, out)
	}

	if strings.TrimSpace(string(out)) != "remote base commit" {
		t.Errorf("expected branch to be based on remote-base, got: %s", out)
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

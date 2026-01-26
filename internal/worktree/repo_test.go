package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFindRepoRoot(t *testing.T) {
	// Create a temp git repo
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Test from repo root
	root, err := FindRepoRoot(repoDir)
	if err != nil {
		t.Fatalf("FindRepoRoot failed: %v", err)
	}
	if root != repoDir {
		t.Errorf("expected %s, got %s", repoDir, root)
	}

	// Test from subdirectory
	subDir := filepath.Join(repoDir, "src", "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	root, err = FindRepoRoot(subDir)
	if err != nil {
		t.Fatalf("FindRepoRoot from subdir failed: %v", err)
	}
	if root != repoDir {
		t.Errorf("expected %s, got %s", repoDir, root)
	}
}

func TestFindRepoRootNotGit(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := FindRepoRoot(tmpDir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestGetRepoName(t *testing.T) {
	name := GetRepoName("/home/user/dev/my-project")
	if name != "my-project" {
		t.Errorf("expected my-project, got %s", name)
	}
}

func TestCreateWorktree(t *testing.T) {
	// Create a temp git repo with initial commit
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize repo with a commit
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Create worktree
	wt := &Manager{
		RepoRoot:     repoDir,
		RepoName:     "myrepo",
		WorktreeBase: worktreeBase,
	}

	wtPath, err := wt.Create("feature-x")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	expectedPath := filepath.Join(worktreeBase, "myrepo", "feature-x")
	if wtPath != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, wtPath)
	}

	// Verify worktree exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory not created")
	}

	// Verify it's a git worktree
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = wtPath
	if err := cmd.Run(); err != nil {
		t.Error("created directory is not a git worktree")
	}
}

func TestCreateWorktreeAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	wt := &Manager{
		RepoRoot:     repoDir,
		RepoName:     "myrepo",
		WorktreeBase: worktreeBase,
	}

	// Create first time
	if _, err := wt.Create("feature-x"); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	// Second create should fail
	_, err := wt.Create("feature-x")
	if err == nil {
		t.Error("expected error for existing worktree")
	}
	if !errors.Is(err, ErrWorktreeExists) {
		t.Errorf("expected ErrWorktreeExists, got %v", err)
	}
}

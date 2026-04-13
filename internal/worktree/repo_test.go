package worktree

import (
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

func TestFindRepoRootFromWorktree(t *testing.T) {
	// Create a temp git repo with initial commit
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeDir := filepath.Join(tmpDir, "worktrees", "myrepo", "feature-x")

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

	// Create a worktree
	if err := os.MkdirAll(filepath.Dir(worktreeDir), 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "worktree", "add", "-b", "feature-x", worktreeDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v\n%s", err, out)
	}

	// FindRepoRoot from within the worktree should return the MAIN repo, not the worktree
	root, err := FindRepoRoot(worktreeDir)
	if err != nil {
		t.Fatalf("FindRepoRoot from worktree failed: %v", err)
	}

	// The bug: currently returns worktreeDir, should return repoDir
	if root != repoDir {
		t.Errorf("FindRepoRoot from worktree: expected main repo %s, got %s", repoDir, root)
	}
}

func TestGetMainBranch(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")

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

	branch, err := GetMainBranch(repoDir)
	if err != nil {
		t.Fatalf("GetMainBranch failed: %v", err)
	}

	// Default branch after git init is usually "master" or "main"
	if branch != "master" && branch != "main" {
		t.Errorf("expected master or main, got %s", branch)
	}
}

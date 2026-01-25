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

func TestGetRepoName(t *testing.T) {
	name := GetRepoName("/home/user/dev/my-project")
	if name != "my-project" {
		t.Errorf("expected my-project, got %s", name)
	}
}

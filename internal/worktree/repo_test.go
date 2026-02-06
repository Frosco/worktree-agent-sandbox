package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
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

func TestGetRepoName(t *testing.T) {
	name := GetRepoName("/home/user/dev/my-project")
	if name != "my-project" {
		t.Errorf("expected my-project, got %s", name)
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

	wtPath, err := wt.Create("feature-x", "")
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
	if _, err := wt.Create("feature-x", ""); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	// Second create should fail
	_, err := wt.Create("feature-x", "")
	if err == nil {
		t.Error("expected error for existing worktree")
	}
	if !errors.Is(err, ErrWorktreeExists) {
		t.Errorf("expected ErrWorktreeExists, got %v", err)
	}
}

func TestListWorktrees(t *testing.T) {
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

	// Create two worktrees
	wt.Create("feature-a", "")
	wt.Create("feature-b", "")

	list, err := wt.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should have 2 worktrees (not counting main)
	if len(list) != 2 {
		t.Errorf("expected 2 worktrees, got %d: %v", len(list), list)
	}
}

func TestRemoveWorktree(t *testing.T) {
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

	// Create and remove
	wtPath, _ := wt.Create("feature-x", "")
	if err := wt.Remove("feature-x", false); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory still exists")
	}
}

func TestCopyConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create source files in repo
	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("# Claude"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "mise.local.toml"), []byte("[tools]"), 0644); err != nil {
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

	wtPath, _ := wt.Create("feature-x", "")

	// Copy files
	filesToCopy := []string{"CLAUDE.md", "mise.local.toml", "nonexistent.txt"}
	copied, err := wt.CopyFiles(wtPath, filesToCopy)
	if err != nil {
		t.Fatalf("CopyFiles failed: %v", err)
	}

	// Should copy 2 files (skip nonexistent)
	if len(copied) != 2 {
		t.Errorf("expected 2 copied files, got %d", len(copied))
	}

	// Verify files exist in worktree
	content, err := os.ReadFile(filepath.Join(wtPath, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md not copied: %v", err)
	}
	if string(content) != "# Claude" {
		t.Errorf("content mismatch: %s", content)
	}
}

func TestDetectConfigChanges(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create source files
	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "unchanged.txt"), []byte("same"), 0644); err != nil {
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

	wtPath, _ := wt.Create("feature-x", "")
	wt.CopyFiles(wtPath, []string{"CLAUDE.md", "unchanged.txt"})

	// Modify one file in worktree
	if err := os.WriteFile(filepath.Join(wtPath, "CLAUDE.md"), []byte("modified in worktree"), 0644); err != nil {
		t.Fatal(err)
	}

	changes, err := wt.DetectChanges(wtPath, []string{"CLAUDE.md", "unchanged.txt"})
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("expected 1 changed file, got %d", len(changes))
	}
	if len(changes) > 0 && changes[0].File != "CLAUDE.md" {
		t.Errorf("expected CLAUDE.md changed, got %s", changes[0].File)
	}
}

func TestDetectConflict(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("original"), 0644); err != nil {
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

	wtPath, _ := wt.Create("feature-x", "")
	wt.CopyFiles(wtPath, []string{"CLAUDE.md"})

	// Modify in both places - add small delay to ensure distinct timestamps
	if err := os.WriteFile(filepath.Join(wtPath, "CLAUDE.md"), []byte("modified in worktree"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // ensure source has later modtime
	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("modified in main"), 0644); err != nil {
		t.Fatal(err)
	}

	changes, _ := wt.DetectChanges(wtPath, []string{"CLAUDE.md"})
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if !changes[0].Conflict {
		t.Error("expected conflict=true")
	}
}

func TestMergeBack(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create source file
	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("original"), 0644); err != nil {
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

	wtPath, _ := wt.Create("feature-x", "")
	wt.CopyFiles(wtPath, []string{"CLAUDE.md"})

	// Modify file in worktree
	if err := os.WriteFile(filepath.Join(wtPath, "CLAUDE.md"), []byte("modified in worktree"), 0644); err != nil {
		t.Fatal(err)
	}

	// Merge back
	result := wt.MergeBack(wtPath, "CLAUDE.md", "feature-x")
	if result.Err != nil {
		t.Fatalf("MergeBack failed: %v", result.Err)
	}

	// Verify source file has worktree content
	content, err := os.ReadFile(filepath.Join(repoDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read merged file: %v", err)
	}
	if string(content) != "modified in worktree" {
		t.Errorf("expected 'modified in worktree', got '%s'", content)
	}
}

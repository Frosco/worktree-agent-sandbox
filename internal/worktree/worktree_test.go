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

func TestCopyFiles_CopiesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	// Create repo and worktree directories
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a directory with files in repo root
	aiDir := filepath.Join(repoRoot, ".ai")
	if err := os.MkdirAll(aiDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiDir, "config.json"), []byte(`{"key": "value"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aiDir, "prompts.txt"), []byte("prompt content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a nested subdirectory
	nestedDir := filepath.Join(aiDir, "templates")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "template1.txt"), []byte("template content"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(repoRoot, worktreeBase)

	// Copy the directory
	copied, err := mgr.CopyFiles(wtPath, []string{".ai"})
	if err != nil {
		t.Fatalf("CopyFiles failed: %v", err)
	}

	// Should report the directory as copied
	if len(copied) != 1 || copied[0] != ".ai" {
		t.Errorf("expected ['.ai'], got %v", copied)
	}

	// Verify all files were copied
	dstConfig := filepath.Join(wtPath, ".ai", "config.json")
	if content, err := os.ReadFile(dstConfig); err != nil || string(content) != `{"key": "value"}` {
		t.Errorf("config.json not copied correctly: %v", err)
	}

	dstPrompts := filepath.Join(wtPath, ".ai", "prompts.txt")
	if content, err := os.ReadFile(dstPrompts); err != nil || string(content) != "prompt content" {
		t.Errorf("prompts.txt not copied correctly: %v", err)
	}

	dstTemplate := filepath.Join(wtPath, ".ai", "templates", "template1.txt")
	if content, err := os.ReadFile(dstTemplate); err != nil || string(content) != "template content" {
		t.Errorf("templates/template1.txt not copied correctly: %v", err)
	}
}

func TestDetectChanges_DetectsDirectoryChanges(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	// Create both directories
	if err := os.MkdirAll(filepath.Join(repoRoot, ".ai"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wtPath, ".ai"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create identical files in both locations initially
	if err := os.WriteFile(filepath.Join(repoRoot, ".ai", "config.json"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, ".ai", "config.json"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	// Modify the file in worktree
	if err := os.WriteFile(filepath.Join(wtPath, ".ai", "config.json"), []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(repoRoot, worktreeBase)

	changes, err := mgr.DetectChanges(wtPath, []string{".ai"})
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	// Should detect the changed file within the directory
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].File != ".ai/config.json" {
		t.Errorf("expected '.ai/config.json', got %q", changes[0].File)
	}
}

func TestDetectChanges_DetectsNewFileInDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	// Create both directories
	if err := os.MkdirAll(filepath.Join(repoRoot, ".ai"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wtPath, ".ai"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file only in the worktree (new file added during work)
	if err := os.WriteFile(filepath.Join(wtPath, ".ai", "new-file.txt"), []byte("new content"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(repoRoot, worktreeBase)

	changes, err := mgr.DetectChanges(wtPath, []string{".ai"})
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	// Should detect the new file
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].File != ".ai/new-file.txt" {
		t.Errorf("expected '.ai/new-file.txt', got %q", changes[0].File)
	}
}

func TestMergeBack_MergesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	// Create directories
	if err := os.MkdirAll(filepath.Join(repoRoot, ".ai"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wtPath, ".ai", "templates"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create files in worktree that should be merged back
	if err := os.WriteFile(filepath.Join(wtPath, ".ai", "config.json"), []byte("updated"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, ".ai", "templates", "new.txt"), []byte("new template"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(repoRoot, worktreeBase)

	// Merge back the directory
	if err := mgr.MergeBack(wtPath, ".ai"); err != nil {
		t.Fatalf("MergeBack failed: %v", err)
	}

	// Verify files were copied to repo root
	dstConfig := filepath.Join(repoRoot, ".ai", "config.json")
	if content, err := os.ReadFile(dstConfig); err != nil || string(content) != "updated" {
		t.Errorf("config.json not merged correctly: %v, content: %s", err, content)
	}

	dstTemplate := filepath.Join(repoRoot, ".ai", "templates", "new.txt")
	if content, err := os.ReadFile(dstTemplate); err != nil || string(content) != "new template" {
		t.Errorf("templates/new.txt not merged correctly: %v", err)
	}
}

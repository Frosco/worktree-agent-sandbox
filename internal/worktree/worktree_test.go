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

func TestBranchUpstream_WithTracking(t *testing.T) {
	mainRepo, bareRemote, worktreeBase := setupRepoWithRemote(t)

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

	mgr := NewManager(mainRepo, worktreeBase)

	upstream := mgr.BranchUpstream("tracked-branch")
	if upstream != "origin/tracked-branch" {
		t.Errorf("expected 'origin/tracked-branch', got %q", upstream)
	}
}

func TestBranchUpstream_NoTracking(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)

	// Create a local-only branch (no push, no tracking)
	cmd := exec.Command("git", "checkout", "-b", "local-only")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo, worktreeBase)

	upstream := mgr.BranchUpstream("local-only")
	if upstream != "" {
		t.Errorf("expected empty string for local-only branch, got %q", upstream)
	}
}

func TestDeleteBranch(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)

	// Create a branch
	cmd := exec.Command("git", "branch", "branch-to-delete")
	cmd.Dir = mainRepo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create branch failed: %v\n%s", err, out)
	}

	mgr := NewManager(mainRepo, worktreeBase)

	// Verify branch exists
	if !mgr.BranchExists("branch-to-delete") {
		t.Fatal("branch should exist before delete")
	}

	// Delete the branch
	err := mgr.DeleteBranch("branch-to-delete")
	if err != nil {
		t.Fatalf("DeleteBranch failed: %v", err)
	}

	// Verify branch is gone
	if mgr.BranchExists("branch-to-delete") {
		t.Error("branch should not exist after delete")
	}
}

func TestDeleteBranch_NotFound(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	err := mgr.DeleteBranch("nonexistent-branch")
	if err == nil {
		t.Error("expected error for nonexistent branch")
	}
}

func TestHasUncommittedChanges_Clean(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	// Create a clean worktree
	wtPath, err := mgr.Create("clean-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if mgr.HasUncommittedChanges(wtPath) {
		t.Error("clean worktree should not have uncommitted changes")
	}
}

func TestHasUncommittedChanges_Modified(t *testing.T) {
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	// Create worktree
	wtPath, err := mgr.Create("modified-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

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
	mainRepo, _, worktreeBase := setupRepoWithRemote(t)
	mgr := NewManager(mainRepo, worktreeBase)

	// Create worktree
	wtPath, err := mgr.Create("staged-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

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

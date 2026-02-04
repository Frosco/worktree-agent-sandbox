package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/niref/wt/internal/worktree"
)

func setupTestRepo(t *testing.T) (repoDir, worktreeBase string) {
	tmpDir := t.TempDir()
	repoDir = filepath.Join(tmpDir, "myrepo")
	worktreeBase = filepath.Join(tmpDir, "worktrees")

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

	return repoDir, worktreeBase
}

func TestNewCommand(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	// Create a CLAUDE.md to copy
	os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("# Claude"), 0644)

	// Create global config
	configDir := filepath.Join(t.TempDir(), "config", "wt")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.toml")
	os.WriteFile(configPath, []byte(`copy_files = ["CLAUDE.md"]`), 0644)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	// Override paths for test
	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	rootCmd.SetArgs([]string{"new", "feature-test",
		"--worktree-base", worktreeBase,
		"--config", configPath,
		"--print-path",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("new command failed: %v\n%s", err, buf.String())
	}

	output := strings.TrimSpace(buf.String())
	expectedPath := filepath.Join(worktreeBase, "myrepo", "feature-test")

	if output != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, output)
	}

	// Verify worktree was created
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("worktree not created")
	}

	// Verify CLAUDE.md was copied
	if _, err := os.Stat(filepath.Join(expectedPath, "CLAUDE.md")); os.IsNotExist(err) {
		t.Error("CLAUDE.md not copied")
	}
}

func TestSwitchCommand(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create the branch first (switch no longer auto-creates branches)
	cmd := exec.Command("git", "branch", "feature-switch")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch failed: %v\n%s", err, out)
	}

	buf := new(bytes.Buffer)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	// First switch creates worktree for existing branch
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"switch", "feature-switch",
		"--worktree-base", worktreeBase,
		"--print-path",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("first switch failed: %v", err)
	}

	expectedPath := filepath.Join(worktreeBase, "myrepo", "feature-switch")
	if strings.TrimSpace(buf.String()) != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, buf.String())
	}

	// Second switch should work (idempotent)
	buf.Reset()
	rootCmd.SetArgs([]string{"switch", "feature-switch",
		"--worktree-base", worktreeBase,
		"--print-path",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("second switch failed: %v", err)
	}

	if strings.TrimSpace(buf.String()) != expectedPath {
		t.Errorf("second switch: expected %s, got %s", expectedPath, buf.String())
	}
}

func TestSwitchNonExistentBranch(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	buf := new(bytes.Buffer)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	// Switch to non-existent branch should fail
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"switch", "typo-branch",
		"--worktree-base", worktreeBase,
		"--print-path",
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("switch to non-existent branch should fail")
	}

	// Error message should mention the branch doesn't exist
	if !strings.Contains(err.Error(), "does not exist") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention branch doesn't exist, got: %v", err)
	}
}

func setupTestRepoWithRemote(t *testing.T) (repoDir, worktreeBase, bareRemote string) {
	t.Helper()
	tmpDir := t.TempDir()
	bareRemote = filepath.Join(tmpDir, "remote.git")
	repoDir = filepath.Join(tmpDir, "local")
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
	cmd = exec.Command("git", "clone", bareRemote, repoDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}

	// Configure and make initial commit
	cmds := [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
		{"git", "push", "-u", "origin", "HEAD"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	return repoDir, worktreeBase, bareRemote
}

func TestSwitchRemoteBranch(t *testing.T) {
	repoDir, worktreeBase, bareRemote := setupTestRepoWithRemote(t)

	// Create a branch in a separate clone and push it
	tmpClone := filepath.Join(t.TempDir(), "tmpclone")
	cmd := exec.Command("git", "clone", bareRemote, tmpClone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v\n%s", err, out)
	}

	cmds := [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "remote-feature"},
		{"git", "commit", "--allow-empty", "-m", "feature commit"},
		{"git", "push", "-u", "origin", "remote-feature"},
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
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fetch failed: %v\n%s", err, out)
	}

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	buf := new(bytes.Buffer)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	// Switch to remote-only branch should work
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"switch", "remote-feature",
		"--worktree-base", worktreeBase,
		"--print-path",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("switch to remote branch failed: %v\n%s", err, buf.String())
	}

	expectedPath := filepath.Join(worktreeBase, "local", "remote-feature")
	output := strings.TrimSpace(buf.String())
	if output != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, output)
	}

	// Verify worktree was created on the correct branch
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = expectedPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch failed: %v\n%s", err, out)
	}

	branch := strings.TrimSpace(string(out))
	if branch != "remote-feature" {
		t.Errorf("expected branch 'remote-feature', got %q", branch)
	}
}

func TestSwitchToMainBranch(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	buf := new(bytes.Buffer)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	// Get the main branch name (could be "main" or "master")
	mainBranch, err := worktree.GetMainBranch(repoDir)
	if err != nil {
		t.Fatalf("GetMainBranch failed: %v", err)
	}

	// Switch to main branch should return the main repo path, not create a worktree
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"switch", mainBranch,
		"--worktree-base", worktreeBase,
		"--print-path",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("switch to main failed: %v\n%s", err, buf.String())
	}

	// Should return the main repo path, not a worktree path
	output := strings.TrimSpace(buf.String())
	if output != repoDir {
		t.Errorf("switch to main: expected main repo %s, got %s", repoDir, output)
	}
}

func TestListCommand(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create some worktrees first
	mgr := worktree.NewManager(repoDir, worktreeBase)
	mgr.Create("feature-a", "")
	mgr.Create("feature-b", "")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"list", "--worktree-base", worktreeBase})
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	}()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "feature-a") {
		t.Error("output should contain feature-a")
	}
	if !strings.Contains(output, "feature-b") {
		t.Error("output should contain feature-b")
	}
}

func TestRemoveCommand(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create a worktree
	mgr := worktree.NewManager(repoDir, worktreeBase)
	wtPath, _ := mgr.Create("feature-remove", "")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"remove", "feature-remove",
		"--worktree-base", worktreeBase,
		"--force", // Skip change detection prompt
	})
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// Verify worktree is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree should be removed")
	}
}

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

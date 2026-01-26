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

	buf := new(bytes.Buffer)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	// First switch creates the worktree
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

func TestListCommand(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create some worktrees first
	mgr := worktree.NewManager(repoDir, worktreeBase)
	mgr.Create("feature-a")
	mgr.Create("feature-b")

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

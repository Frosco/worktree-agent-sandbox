package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "wt-test")
	cmd := exec.Command("go", "build", "-o", binary, "./")
	cmd.Dir = filepath.Join(mustFindProjectRoot(t), "cmd", "wt")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binary
}

func mustFindProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from current file to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

func setupSwitchTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	bare := filepath.Join(tmpDir, "remote.git")
	repo := filepath.Join(tmpDir, "local")

	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		dir  string
		args []string
	}{
		{bare, []string{"git", "init", "--bare"}},
		{tmpDir, []string{"git", "clone", bare, repo}},
		{repo, []string{"git", "config", "user.email", "test@test.com"}},
		{repo, []string{"git", "config", "user.name", "Test"}},
		{repo, []string{"git", "commit", "--allow-empty", "-m", "initial"}},
		{repo, []string{"git", "push", "-u", "origin", "HEAD"}},
	} {
		cmd := exec.Command(c.args[0], c.args[1:]...)
		cmd.Dir = c.dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v in %s failed: %v\n%s", c.args, c.dir, err, out)
		}
	}

	return repo
}

func pushRemoteBranch(t *testing.T, repo, branch string) {
	t.Helper()
	for _, args := range [][]string{
		{"git", "checkout", "-b", branch},
		{"git", "commit", "--allow-empty", "-m", "commit on " + branch},
		{"git", "push", "origin", branch},
		{"git", "checkout", "main"},
		{"git", "branch", "-D", branch},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestSwitch_CreatesFromRemoteBranch(t *testing.T) {
	binary := buildBinary(t)
	repo := setupSwitchTestRepo(t)
	pushRemoteBranch(t, repo, "feature-review")

	// Run wt switch feature-review --print-path
	cmd := exec.Command(binary, "switch", "--print-path", "feature-review")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("switch failed: %v\n%s", err, out)
	}

	wtPath := strings.TrimSpace(string(out))
	expectedPath := filepath.Join(repo, ".claude", "worktrees", "feature-review")
	if wtPath != expectedPath {
		t.Errorf("output path = %q, want %q", wtPath, expectedPath)
	}

	// Verify worktree directory was created
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("worktree not created at %s", expectedPath)
	}

	// Verify correct branch is checked out
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = expectedPath
	branchOut, err := branchCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse failed: %v\n%s", err, branchOut)
	}
	if branch := strings.TrimSpace(string(branchOut)); branch != "feature-review" {
		t.Errorf("branch = %q, want %q", branch, "feature-review")
	}
}

func TestSwitch_NoWorktreeNoRemote_Errors(t *testing.T) {
	binary := buildBinary(t)
	repo := setupSwitchTestRepo(t)

	cmd := exec.Command(binary, "switch", "nonexistent")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for nonexistent worktree and no remote branch")
	}

	output := string(out)
	if !strings.Contains(output, "no worktree or remote branch") {
		t.Errorf("error output %q should mention 'no worktree or remote branch'", output)
	}
}

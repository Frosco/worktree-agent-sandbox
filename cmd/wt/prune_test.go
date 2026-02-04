package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/niref/wt/internal/worktree"
)

func TestPrune_DryRun_ShowsCandidates(t *testing.T) {
	repoDir, worktreeBase, bareRemote := setupTestRepoWithRemote(t)

	// Create a branch, push it, create worktree, then delete from remote
	cmds := [][]string{
		{"git", "checkout", "-b", "gone-branch"},
		{"git", "commit", "--allow-empty", "-m", "gone commit"},
		{"git", "push", "-u", "origin", "gone-branch"},
		{"git", "checkout", "master"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Create worktree for the branch
	mgr := worktree.NewManager(repoDir, worktreeBase)
	_, err := mgr.Create("gone-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete branch from remote (simulating merge)
	cmd := exec.Command("git", "push", "origin", "--delete", "gone-branch")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("push delete failed: %v\n%s", err, out)
	}
	_ = bareRemote

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
		pruneDryRun = false
		pruneNoFetch = false
	}()

	rootCmd.SetArgs([]string{"prune", "--dry-run",
		"--worktree-base", worktreeBase,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune --dry-run failed: %v\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "gone-branch") {
		t.Errorf("output should mention gone-branch, got: %s", output)
	}
	if !strings.Contains(output, "Would prune") || !strings.Contains(output, "dry-run") {
		t.Errorf("output should indicate dry-run mode, got: %s", output)
	}

	// Worktree should still exist (dry-run)
	if !mgr.Exists("gone-branch") {
		t.Error("worktree should still exist after dry-run")
	}
}

func TestPrune_NothingToPrune(t *testing.T) {
	repoDir, worktreeBase, _ := setupTestRepoWithRemote(t)

	// Create a branch that still exists on remote
	cmds := [][]string{
		{"git", "checkout", "-b", "active-branch"},
		{"git", "commit", "--allow-empty", "-m", "active commit"},
		{"git", "push", "-u", "origin", "active-branch"},
		{"git", "checkout", "master"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Create worktree
	mgr := worktree.NewManager(repoDir, worktreeBase)
	_, err := mgr.Create("active-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

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
		pruneDryRun = false
		pruneNoFetch = false
	}()

	rootCmd.SetArgs([]string{"prune",
		"--worktree-base", worktreeBase,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune failed: %v\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "Nothing to prune") {
		t.Errorf("output should say nothing to prune, got: %s", output)
	}
}

func TestPrune_SkipsLocalOnlyBranches(t *testing.T) {
	repoDir, worktreeBase, _ := setupTestRepoWithRemote(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create a local-only branch (never pushed, no tracking)
	mgr := worktree.NewManager(repoDir, worktreeBase)
	_, err := mgr.Create("local-only-branch", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
		pruneDryRun = false
		pruneNoFetch = false
	}()

	rootCmd.SetArgs([]string{"prune", "--dry-run",
		"--worktree-base", worktreeBase,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune failed: %v\n%s", err, buf.String())
	}

	output := buf.String()
	// Should NOT mention local-only-branch as a prune candidate
	if strings.Contains(output, "local-only-branch") && !strings.Contains(output, "Nothing to prune") {
		t.Errorf("local-only branch should not be a prune candidate, got: %s", output)
	}

	// Worktree should still exist
	if !mgr.Exists("local-only-branch") {
		t.Error("local-only worktree should not be pruned")
	}
}

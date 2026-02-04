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

func TestPrune_RemovesGoneBranch(t *testing.T) {
	repoDir, worktreeBase, _ := setupTestRepoWithRemote(t)

	// Create a branch, push it, create worktree, then delete from remote
	cmds := [][]string{
		{"git", "checkout", "-b", "to-prune"},
		{"git", "commit", "--allow-empty", "-m", "prune me"},
		{"git", "push", "-u", "origin", "to-prune"},
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
	_, err := mgr.Create("to-prune", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete branch from remote
	cmd := exec.Command("git", "push", "origin", "--delete", "to-prune")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("push delete failed: %v\n%s", err, out)
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
		pruneForce = false
		pruneSkipChanges = false
	}()

	// Use --force and --skip-changes to avoid prompts
	rootCmd.SetArgs([]string{"prune",
		"--worktree-base", worktreeBase,
		"--force",
		"--skip-changes",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune failed: %v\n%s", err, buf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "to-prune") {
		t.Errorf("output should mention pruned branch, got: %s", output)
	}
	if !strings.Contains(output, "Pruned") {
		t.Errorf("output should confirm pruning, got: %s", output)
	}

	// Worktree should be gone
	if mgr.Exists("to-prune") {
		t.Error("worktree should be removed after prune")
	}

	// Local branch should be gone
	if mgr.BranchExists("to-prune") {
		t.Error("local branch should be deleted after prune")
	}
}

func TestPrune_MultipleWorktrees(t *testing.T) {
	repoDir, worktreeBase, _ := setupTestRepoWithRemote(t)

	// Create multiple branches
	branches := []string{"prune-a", "prune-b", "keep-c"}
	for _, branch := range branches {
		cmds := [][]string{
			{"git", "checkout", "-b", branch},
			{"git", "commit", "--allow-empty", "-m", branch + " commit"},
			{"git", "push", "-u", "origin", branch},
			{"git", "checkout", "master"},
		}
		for _, args := range cmds {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = repoDir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v failed: %v\n%s", args, err, out)
			}
		}
	}

	// Create worktrees for all
	mgr := worktree.NewManager(repoDir, worktreeBase)
	for _, branch := range branches {
		_, err := mgr.Create(branch, "")
		if err != nil {
			t.Fatalf("Create %s failed: %v", branch, err)
		}
	}

	// Delete prune-a and prune-b from remote, keep keep-c
	for _, branch := range []string{"prune-a", "prune-b"} {
		cmd := exec.Command("git", "push", "origin", "--delete", branch)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("push delete %s failed: %v\n%s", branch, err, out)
		}
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
		pruneForce = false
		pruneSkipChanges = false
	}()

	rootCmd.SetArgs([]string{"prune",
		"--worktree-base", worktreeBase,
		"--force",
		"--skip-changes",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("prune failed: %v\n%s", err, buf.String())
	}

	output := buf.String()

	// Should have pruned a and b
	if !strings.Contains(output, "prune-a") {
		t.Errorf("output should mention prune-a, got: %s", output)
	}
	if !strings.Contains(output, "prune-b") {
		t.Errorf("output should mention prune-b, got: %s", output)
	}

	// prune-a and prune-b worktrees should be gone
	if mgr.Exists("prune-a") {
		t.Error("prune-a worktree should be removed")
	}
	if mgr.Exists("prune-b") {
		t.Error("prune-b worktree should be removed")
	}

	// keep-c should still exist
	if !mgr.Exists("keep-c") {
		t.Error("keep-c worktree should still exist")
	}
}

package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMemorySnapshotPath(t *testing.T) {
	mgr := &Manager{
		RepoRoot:     "/repo",
		RepoName:     "myrepo",
		WorktreeBase: "/data/wt/worktrees",
	}

	got := mgr.MemorySnapshotPath("feature-x")
	expected := "/data/wt/snapshots/myrepo/feature-x/claude-memory"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestCopyMemory(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	os.MkdirAll(wtPath, 0755)

	// Create main's Claude memory directory
	mainMemDir, _ := ClaudeMemoryDir(repoRoot)
	t.Cleanup(func() {
		// ClaudeMemoryDir resolves under ~/.claude/projects, clean up after test
		os.RemoveAll(mainMemDir)
	})
	os.MkdirAll(mainMemDir, 0755)
	os.WriteFile(filepath.Join(mainMemDir, "MEMORY.md"), []byte("# Memory\nKey insight"), 0644)
	os.WriteFile(filepath.Join(mainMemDir, "debugging.md"), []byte("# Debugging notes"), 0644)

	mgr := NewManager(repoRoot, worktreeBase)

	if err := mgr.CopyMemory(wtPath); err != nil {
		t.Fatalf("CopyMemory failed: %v", err)
	}

	// Verify files were copied to worktree's Claude memory dir
	wtMemDir, _ := ClaudeMemoryDir(wtPath)
	t.Cleanup(func() {
		os.RemoveAll(wtMemDir)
	})

	content, err := os.ReadFile(filepath.Join(wtMemDir, "MEMORY.md"))
	if err != nil {
		t.Fatalf("MEMORY.md not copied: %v", err)
	}
	if string(content) != "# Memory\nKey insight" {
		t.Errorf("content mismatch: %s", content)
	}

	content, err = os.ReadFile(filepath.Join(wtMemDir, "debugging.md"))
	if err != nil {
		t.Fatalf("debugging.md not copied: %v", err)
	}
	if string(content) != "# Debugging notes" {
		t.Errorf("content mismatch: %s", content)
	}
}

func TestCopyMemory_NoMainMemory(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	os.MkdirAll(wtPath, 0755)

	mgr := NewManager(repoRoot, worktreeBase)

	// Should not error when main has no memory
	if err := mgr.CopyMemory(wtPath); err != nil {
		t.Fatalf("CopyMemory should be no-op when no memory exists: %v", err)
	}

	// Verify worktree memory dir was NOT created
	wtMemDir, _ := ClaudeMemoryDir(wtPath)
	if _, err := os.Stat(wtMemDir); !os.IsNotExist(err) {
		t.Error("worktree memory dir should not exist when main has none")
	}
}

func TestSaveMemorySnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)

	// Create main's Claude memory
	mainMemDir, _ := ClaudeMemoryDir(repoRoot)
	t.Cleanup(func() { os.RemoveAll(mainMemDir) })
	os.MkdirAll(mainMemDir, 0755)
	os.WriteFile(filepath.Join(mainMemDir, "MEMORY.md"), []byte("# Memory"), 0644)

	mgr := NewManager(repoRoot, worktreeBase)

	if err := mgr.SaveMemorySnapshot("feature-x"); err != nil {
		t.Fatalf("SaveMemorySnapshot failed: %v", err)
	}

	// Verify snapshot exists
	snapshotPath := mgr.MemorySnapshotPath("feature-x")
	content, err := os.ReadFile(filepath.Join(snapshotPath, "MEMORY.md"))
	if err != nil {
		t.Fatalf("snapshot not created: %v", err)
	}
	if string(content) != "# Memory" {
		t.Errorf("snapshot content mismatch: %s", content)
	}
}

func TestSaveMemorySnapshot_NoMainMemory(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)

	mgr := NewManager(repoRoot, worktreeBase)

	// Should not error when no memory exists
	if err := mgr.SaveMemorySnapshot("feature-x"); err != nil {
		t.Fatalf("should be no-op: %v", err)
	}
}

func TestRemoveMemorySnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)

	mainMemDir, _ := ClaudeMemoryDir(repoRoot)
	t.Cleanup(func() { os.RemoveAll(mainMemDir) })
	os.MkdirAll(mainMemDir, 0755)
	os.WriteFile(filepath.Join(mainMemDir, "MEMORY.md"), []byte("# Memory"), 0644)

	mgr := NewManager(repoRoot, worktreeBase)

	mgr.SaveMemorySnapshot("feature-x")

	snapshotPath := mgr.MemorySnapshotPath("feature-x")
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		t.Fatal("snapshot should exist before removal")
	}

	if err := mgr.RemoveMemorySnapshot("feature-x"); err != nil {
		t.Fatalf("RemoveMemorySnapshot failed: %v", err)
	}

	if _, err := os.Stat(snapshotPath); !os.IsNotExist(err) {
		t.Error("snapshot should be removed")
	}
}

func TestRemoveMemorySnapshot_NonexistentIsNotError(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)

	mgr := NewManager(repoRoot, worktreeBase)

	if err := mgr.RemoveMemorySnapshot("nonexistent"); err != nil {
		t.Errorf("should not error: %v", err)
	}
}

func TestDetectMemoryChanges_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	os.MkdirAll(wtPath, 0755)

	// Create identical memory in both
	mainMemDir, _ := ClaudeMemoryDir(repoRoot)
	wtMemDir, _ := ClaudeMemoryDir(wtPath)
	t.Cleanup(func() {
		os.RemoveAll(mainMemDir)
		os.RemoveAll(wtMemDir)
	})
	os.MkdirAll(mainMemDir, 0755)
	os.MkdirAll(wtMemDir, 0755)
	os.WriteFile(filepath.Join(mainMemDir, "MEMORY.md"), []byte("# Same"), 0644)
	os.WriteFile(filepath.Join(wtMemDir, "MEMORY.md"), []byte("# Same"), 0644)

	mgr := NewManager(repoRoot, worktreeBase)

	changes, err := mgr.DetectMemoryChanges(wtPath, "feature-x")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d", len(changes))
	}
}

func TestDetectMemoryChanges_Modified(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	os.MkdirAll(wtPath, 0755)

	mainMemDir, _ := ClaudeMemoryDir(repoRoot)
	wtMemDir, _ := ClaudeMemoryDir(wtPath)
	t.Cleanup(func() {
		os.RemoveAll(mainMemDir)
		os.RemoveAll(wtMemDir)
	})
	os.MkdirAll(mainMemDir, 0755)
	os.MkdirAll(wtMemDir, 0755)
	os.WriteFile(filepath.Join(mainMemDir, "MEMORY.md"), []byte("# Original"), 0644)
	os.WriteFile(filepath.Join(wtMemDir, "MEMORY.md"), []byte("# Modified by Claude"), 0644)

	mgr := NewManager(repoRoot, worktreeBase)

	changes, err := mgr.DetectMemoryChanges(wtPath, "feature-x")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].File != "MEMORY.md" {
		t.Errorf("expected MEMORY.md, got %q", changes[0].File)
	}
}

func TestDetectMemoryChanges_MainHasNoMemory(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	os.MkdirAll(wtPath, 0755)

	// Only worktree has memory
	wtMemDir, _ := ClaudeMemoryDir(wtPath)
	t.Cleanup(func() {
		os.RemoveAll(wtMemDir)
	})
	os.MkdirAll(wtMemDir, 0755)
	os.WriteFile(filepath.Join(wtMemDir, "MEMORY.md"), []byte("# New memory"), 0644)

	mgr := NewManager(repoRoot, worktreeBase)

	changes, err := mgr.DetectMemoryChanges(wtPath, "feature-x")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].File != "MEMORY.md" {
		t.Errorf("expected MEMORY.md, got %q", changes[0].File)
	}
	if changes[0].Conflict {
		t.Error("should not be a conflict when main has no memory")
	}
}

func TestDetectMemoryChanges_WorktreeHasNoMemory(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	os.MkdirAll(wtPath, 0755)

	mgr := NewManager(repoRoot, worktreeBase)

	changes, err := mgr.DetectMemoryChanges(wtPath, "feature-x")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d", len(changes))
	}
}

func TestMergeMemoryBack_FallbackCopy(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	os.MkdirAll(wtPath, 0755)

	// Create memory in both (no snapshot → fallback to copy)
	mainMemDir, _ := ClaudeMemoryDir(repoRoot)
	wtMemDir, _ := ClaudeMemoryDir(wtPath)
	t.Cleanup(func() {
		os.RemoveAll(mainMemDir)
		os.RemoveAll(wtMemDir)
	})
	os.MkdirAll(mainMemDir, 0755)
	os.MkdirAll(wtMemDir, 0755)
	os.WriteFile(filepath.Join(mainMemDir, "MEMORY.md"), []byte("main version"), 0644)
	os.WriteFile(filepath.Join(wtMemDir, "MEMORY.md"), []byte("worktree version"), 0644)

	mgr := NewManager(repoRoot, worktreeBase)

	result := mgr.MergeMemoryBack(wtPath, "MEMORY.md", "feature-x")
	if result.Status != MergeStatusCopied {
		t.Errorf("expected MergeStatusCopied, got %v", result.Status)
	}

	content, _ := os.ReadFile(filepath.Join(mainMemDir, "MEMORY.md"))
	if string(content) != "worktree version" {
		t.Errorf("expected worktree version, got %q", string(content))
	}
}

func TestMergeMemoryBack_ThreeWayCleanMerge(t *testing.T) {
	if _, err := exec.LookPath("mergiraf"); err != nil {
		t.Skip("mergiraf not available")
	}

	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	os.MkdirAll(wtPath, 0755)

	mainMemDir, _ := ClaudeMemoryDir(repoRoot)
	wtMemDir, _ := ClaudeMemoryDir(wtPath)
	t.Cleanup(func() {
		os.RemoveAll(mainMemDir)
		os.RemoveAll(wtMemDir)
	})
	os.MkdirAll(mainMemDir, 0755)
	os.MkdirAll(wtMemDir, 0755)

	mgr := NewManager(repoRoot, worktreeBase)

	base := "line1\nline2\nline3\nline4\nline5\n"
	left := "modified1\nline2\nline3\nline4\nline5\n"
	right := "line1\nline2\nline3\nline4\nmodified5\n"

	// Write base and snapshot it
	os.WriteFile(filepath.Join(mainMemDir, "MEMORY.md"), []byte(base), 0644)
	mgr.SaveMemorySnapshot("feature-x")

	// Modify both sides
	os.WriteFile(filepath.Join(mainMemDir, "MEMORY.md"), []byte(left), 0644)
	os.WriteFile(filepath.Join(wtMemDir, "MEMORY.md"), []byte(right), 0644)

	result := mgr.MergeMemoryBack(wtPath, "MEMORY.md", "feature-x")
	if result.Status != MergeStatusMerged {
		t.Errorf("expected MergeStatusMerged, got %v (err: %v)", result.Status, result.Err)
	}

	content, _ := os.ReadFile(filepath.Join(mainMemDir, "MEMORY.md"))
	expected := "modified1\nline2\nline3\nline4\nmodified5\n"
	if string(content) != expected {
		t.Errorf("merged content mismatch:\n  got:  %q\n  want: %q", string(content), expected)
	}
}

func TestMergeMemoryBack_MainNoMemoryDir(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktree")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	os.MkdirAll(wtPath, 0755)

	// Only worktree has memory
	wtMemDir, _ := ClaudeMemoryDir(wtPath)
	t.Cleanup(func() {
		os.RemoveAll(wtMemDir)
	})
	os.MkdirAll(wtMemDir, 0755)
	os.WriteFile(filepath.Join(wtMemDir, "MEMORY.md"), []byte("new memory"), 0644)

	mgr := NewManager(repoRoot, worktreeBase)

	result := mgr.MergeMemoryBack(wtPath, "MEMORY.md", "feature-x")
	if result.Status != MergeStatusCopied {
		t.Errorf("expected MergeStatusCopied, got %v", result.Status)
	}

	// Verify main's memory dir was created with the content
	mainMemDir, _ := ClaudeMemoryDir(repoRoot)
	t.Cleanup(func() {
		os.RemoveAll(mainMemDir)
	})
	content, err := os.ReadFile(filepath.Join(mainMemDir, "MEMORY.md"))
	if err != nil {
		t.Fatalf("main memory should be created: %v", err)
	}
	if string(content) != "new memory" {
		t.Errorf("expected 'new memory', got %q", string(content))
	}
}

func TestClaudeMemoryDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "/home/user/dev/my-project",
			expected: filepath.Join(home, ".claude", "projects", "-home-user-dev-my-project", "memory"),
		},
		{
			name:     "path with dots",
			path:     "/home/user/.local/share/wt/worktrees/repo/branch",
			expected: filepath.Join(home, ".claude", "projects", "-home-user--local-share-wt-worktrees-repo-branch", "memory"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClaudeMemoryDir(tt.path)
			if err != nil {
				t.Fatalf("ClaudeMemoryDir(%q) error: %v", tt.path, err)
			}
			if got != tt.expected {
				t.Errorf("ClaudeMemoryDir(%q)\n  got:  %s\n  want: %s", tt.path, got, tt.expected)
			}
		})
	}
}

package worktree

import (
	"os"
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

package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

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

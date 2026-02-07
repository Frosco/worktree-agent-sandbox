package worktree

import (
	"os"
	"path/filepath"
	"strings"
)

// ClaudeMemoryDir returns the path to Claude Code's memory directory for a project.
// Claude Code encodes paths by stripping the leading /, replacing / and . with -, and prepending -.
func ClaudeMemoryDir(projectPath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	encoded := encodeClaudePath(projectPath)
	return filepath.Join(home, ".claude", "projects", encoded, "memory"), nil
}

// MemorySnapshotPath returns the path where memory snapshots are stored for a branch.
// This is a subdirectory within the existing snapshot hierarchy.
func (m *Manager) MemorySnapshotPath(branch string) string {
	return filepath.Join(m.SnapshotPath(branch), "claude-memory")
}

func encodeClaudePath(path string) string {
	// Strip leading /
	path = strings.TrimPrefix(path, "/")
	// Replace / and . with -
	var b strings.Builder
	b.WriteByte('-') // prepend -
	for _, c := range path {
		if c == '/' || c == '.' {
			b.WriteByte('-')
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

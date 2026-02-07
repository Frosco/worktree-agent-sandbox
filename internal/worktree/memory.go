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

// CopyMemory copies the main repo's Claude memory directory to the worktree's
// Claude memory location. No-op if main has no memory directory.
func (m *Manager) CopyMemory(wtPath string) error {
	srcDir, err := ClaudeMemoryDir(m.RepoRoot)
	if err != nil {
		return err
	}

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil // no memory to copy
	} else if err != nil {
		return err
	}

	// Check if directory has any content
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	dstDir, err := ClaudeMemoryDir(wtPath)
	if err != nil {
		return err
	}

	return copyDir(srcDir, dstDir)
}

// SaveMemorySnapshot saves a copy of main's Claude memory directory to the
// snapshot directory. No-op if main has no memory.
func (m *Manager) SaveMemorySnapshot(branch string) error {
	srcDir, err := ClaudeMemoryDir(m.RepoRoot)
	if err != nil {
		return err
	}

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	dstDir := m.MemorySnapshotPath(branch)
	return copyDir(srcDir, dstDir)
}

// RemoveMemorySnapshot deletes the memory snapshot directory for a branch.
// Returns nil if it doesn't exist.
func (m *Manager) RemoveMemorySnapshot(branch string) error {
	snapshotDir := m.MemorySnapshotPath(branch)
	err := os.RemoveAll(snapshotDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DetectMemoryChanges compares the worktree's Claude memory directory with main's.
// Returns changes for files that differ. File paths are relative to the memory directory.
func (m *Manager) DetectMemoryChanges(wtPath, branch string) ([]FileChange, error) {
	wtMemDir, err := ClaudeMemoryDir(wtPath)
	if err != nil {
		return nil, err
	}

	// If worktree has no memory directory, nothing to detect
	if _, err := os.Stat(wtMemDir); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	mainMemDir, err := ClaudeMemoryDir(m.RepoRoot)
	if err != nil {
		return nil, err
	}

	// Walk worktree memory dir, compare each file with main
	var changes []FileChange

	err = filepath.Walk(wtMemDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(wtMemDir, path)
		if err != nil {
			return err
		}

		mainPath := filepath.Join(mainMemDir, relPath)
		change, hasChange, detectErr := m.detectFileChange(mainPath, path, relPath)
		if detectErr != nil {
			return detectErr
		}
		if hasChange {
			changes = append(changes, change)
		}

		return nil
	})

	return changes, err
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

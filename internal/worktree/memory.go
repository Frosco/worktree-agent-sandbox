package worktree

import (
	"os"
	"os/exec"
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

// MergeMemoryBack merges a memory file from the worktree's Claude memory dir
// back to main's Claude memory dir. Uses three-way merge when snapshot + mergiraf
// are available, otherwise falls back to plain copy.
func (m *Manager) MergeMemoryBack(wtPath, file, branch string) MergeResult {
	wtMemDir, err := ClaudeMemoryDir(wtPath)
	if err != nil {
		return MergeResult{Status: MergeStatusError, Err: err}
	}
	mainMemDir, err := ClaudeMemoryDir(m.RepoRoot)
	if err != nil {
		return MergeResult{Status: MergeStatusError, Err: err}
	}

	srcPath := filepath.Join(wtMemDir, file)
	dstPath := filepath.Join(mainMemDir, file)

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return MergeResult{Status: MergeStatusError, Err: err}
	}

	// Directories always use copy
	if srcInfo.IsDir() {
		if err := copyDir(srcPath, dstPath); err != nil {
			return MergeResult{Status: MergeStatusError, Err: err}
		}
		return MergeResult{Status: MergeStatusCopied}
	}

	// Try three-way merge if snapshot exists and mergiraf is available
	snapshotFile := filepath.Join(m.MemorySnapshotPath(branch), file)
	if _, err := os.Stat(snapshotFile); err == nil {
		if mergirafPath, err := exec.LookPath("mergiraf"); err == nil {
			return m.mergeThreeWay(mergirafPath, snapshotFile, dstPath, srcPath)
		}
	}

	// Fallback: plain copy (also handles "main has no memory" case)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return MergeResult{Status: MergeStatusError, Err: err}
	}
	if err := copyFile(srcPath, dstPath); err != nil {
		return MergeResult{Status: MergeStatusError, Err: err}
	}
	return MergeResult{Status: MergeStatusCopied}
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

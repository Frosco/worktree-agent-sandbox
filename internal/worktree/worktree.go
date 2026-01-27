package worktree

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrWorktreeExists = errors.New("worktree already exists")
var ErrWorktreeNotFound = errors.New("worktree does not exist")
var ErrBranchNotFound = errors.New("branch does not exist")

// Manager handles worktree operations for a repository
type Manager struct {
	RepoRoot     string
	RepoName     string
	WorktreeBase string
}

// NewManager creates a Manager for the repo at the given root
func NewManager(repoRoot, worktreeBase string) *Manager {
	return &Manager{
		RepoRoot:     repoRoot,
		RepoName:     GetRepoName(repoRoot),
		WorktreeBase: worktreeBase,
	}
}

// WorktreePath returns the path where a branch's worktree would be located
func (m *Manager) WorktreePath(branch string) string {
	return filepath.Join(m.WorktreeBase, m.RepoName, branch)
}

// Exists checks if a worktree for the branch already exists
func (m *Manager) Exists(branch string) bool {
	wtPath := m.WorktreePath(branch)
	_, err := os.Stat(wtPath)
	return err == nil
}

// BranchExists checks if a local branch exists in the git repository
func (m *Manager) BranchExists(branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = m.RepoRoot
	return cmd.Run() == nil
}

// RemoteBranchExists checks if a branch exists on the origin remote
func (m *Manager) RemoteBranchExists(branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/remotes/origin/"+branch)
	cmd.Dir = m.RepoRoot
	return cmd.Run() == nil
}

// Create creates a new worktree for the given branch.
// If the branch exists locally, uses it directly.
// If the branch exists only on origin, creates a local tracking branch.
// If the branch doesn't exist anywhere, creates a new branch.
func (m *Manager) Create(branch string) (string, error) {
	wtPath := m.WorktreePath(branch)

	if m.Exists(branch) {
		return "", ErrWorktreeExists
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", err
	}

	localExists := m.BranchExists(branch)
	remoteExists := m.RemoteBranchExists(branch)

	var cmd *exec.Cmd
	switch {
	case localExists:
		// Local branch exists - use it directly
		cmd = exec.Command("git", "worktree", "add", wtPath, branch)
	case remoteExists:
		// Remote branch exists - create local tracking branch
		cmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath, "origin/"+branch)
	default:
		// No branch exists - create new branch
		cmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	}
	cmd.Dir = m.RepoRoot

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return wtPath, nil
}

// WorktreeInfo holds information about a worktree
type WorktreeInfo struct {
	Path   string
	Branch string
}

// List returns all worktrees managed by wt for this repo
func (m *Manager) List() ([]WorktreeInfo, error) {
	repoWorktreeDir := filepath.Join(m.WorktreeBase, m.RepoName)

	entries, err := os.ReadDir(repoWorktreeDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var worktrees []WorktreeInfo
	for _, entry := range entries {
		if entry.IsDir() {
			wtPath := filepath.Join(repoWorktreeDir, entry.Name())
			worktrees = append(worktrees, WorktreeInfo{
				Path:   wtPath,
				Branch: entry.Name(),
			})
		}
	}

	return worktrees, nil
}

// Remove removes a worktree by branch name
func (m *Manager) Remove(branch string) error {
	wtPath := m.WorktreePath(branch)

	if !m.Exists(branch) {
		return ErrWorktreeNotFound
	}

	cmd := exec.Command("git", "worktree", "remove", wtPath)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// CopyFiles copies files from repo root to worktree.
// Skips files that don't exist in the source.
// Returns list of files that were copied.
func (m *Manager) CopyFiles(wtPath string, files []string) ([]string, error) {
	var copied []string

	for _, file := range files {
		srcPath := filepath.Join(m.RepoRoot, file)
		dstPath := filepath.Join(wtPath, file)

		// Skip if source doesn't exist
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return copied, err
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return copied, err
		}

		copied = append(copied, file)
	}

	return copied, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return dstFile.Close()
}

// FileChange represents a changed config file
type FileChange struct {
	File     string
	Conflict bool // true if source also changed
}

// DetectChanges checks if config files in worktree differ from source.
// Also detects conflicts where source changed too.
func (m *Manager) DetectChanges(wtPath string, files []string) ([]FileChange, error) {
	var changes []FileChange

	for _, file := range files {
		srcPath := filepath.Join(m.RepoRoot, file)
		dstPath := filepath.Join(wtPath, file)

		// Skip if file doesn't exist in worktree
		dstContent, err := os.ReadFile(dstPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}

		srcContent, err := os.ReadFile(srcPath)
		if os.IsNotExist(err) {
			// File exists in worktree but not source - that's a change
			changes = append(changes, FileChange{File: file, Conflict: false})
			continue
		}
		if err != nil {
			return nil, err
		}

		// Compare contents
		if !bytes.Equal(srcContent, dstContent) {
			// Check if this is a conflict (source also differs from what we copied)
			// We detect conflict by checking if source hash differs from destination
			// In a real implementation, we'd store the original hash when copying
			// For now, we'll check if source differs from worktree content
			change := FileChange{File: file, Conflict: false}

			// Simple conflict detection: if both differ, it's a conflict
			// This is imperfect but catches the common case
			// Check if source was modified after copy by comparing mod times
			srcInfo, _ := os.Stat(srcPath)
			dstInfo, _ := os.Stat(dstPath)
			if srcInfo != nil && dstInfo != nil {
				// If source is newer than destination, likely a conflict
				if srcInfo.ModTime().After(dstInfo.ModTime()) {
					change.Conflict = true
				}
			}

			changes = append(changes, change)
		}
	}

	return changes, nil
}

// MergeBack copies a file from worktree back to source repo
func (m *Manager) MergeBack(wtPath, file string) error {
	srcPath := filepath.Join(wtPath, file)
	dstPath := filepath.Join(m.RepoRoot, file)
	return copyFile(srcPath, dstPath)
}

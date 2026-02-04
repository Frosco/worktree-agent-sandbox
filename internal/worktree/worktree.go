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
var ErrBaseBranchNotFound = errors.New("base branch not found")

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

// BranchUpstream returns the upstream tracking ref for a branch (e.g., "origin/main").
// Returns empty string if the branch has no upstream configured.
func (m *Manager) BranchUpstream(branch string) string {
	cmd := exec.Command("git", "for-each-ref", "--format=%(upstream:short)", "refs/heads/"+branch)
	cmd.Dir = m.RepoRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// FetchBranch fetches a specific branch from origin.
// Returns an error if the branch doesn't exist on the remote.
func (m *Manager) FetchBranch(branch string) error {
	cmd := exec.Command("git", "fetch", "origin", branch)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch origin %s: %w: %s", branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Create creates a new worktree for the given branch.
// If baseBranch is specified, creates a new branch based on it.
// If baseBranch is empty:
//   - If the branch exists locally, uses it directly.
//   - If the branch exists only on origin, creates a local tracking branch.
//   - If the branch doesn't exist anywhere, creates a new branch from HEAD.
func (m *Manager) Create(branch, baseBranch string) (string, error) {
	wtPath := m.WorktreePath(branch)

	if m.Exists(branch) {
		return "", ErrWorktreeExists
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", err
	}

	// If base branch specified, resolve it and create new branch based on it
	if baseBranch != "" {
		baseRef := baseBranch
		if !m.BranchExists(baseBranch) {
			// Try to fetch from origin
			if err := m.FetchBranch(baseBranch); err != nil {
				return "", ErrBaseBranchNotFound
			}
			if !m.RemoteBranchExists(baseBranch) {
				return "", ErrBaseBranchNotFound
			}
			baseRef = "origin/" + baseBranch
		}
		cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, baseRef)
		cmd.Dir = m.RepoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return wtPath, nil
	}

	// No base branch - use existing behavior
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

// Remove removes a worktree by branch name.
// If force is true, removes even if worktree has uncommitted changes.
func (m *Manager) Remove(branch string, force bool) error {
	wtPath := m.WorktreePath(branch)

	if !m.Exists(branch) {
		return ErrWorktreeNotFound
	}

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wtPath)

	cmd := exec.Command("git", args...)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// CopyFiles copies files or directories from repo root to worktree.
// Skips entries that don't exist in the source.
// Returns list of entries that were copied.
func (m *Manager) CopyFiles(wtPath string, files []string) ([]string, error) {
	var copied []string

	for _, file := range files {
		srcPath := filepath.Join(m.RepoRoot, file)
		dstPath := filepath.Join(wtPath, file)

		srcInfo, err := os.Stat(srcPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return copied, err
		}

		if srcInfo.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return copied, err
			}
		} else {
			// Ensure destination directory exists
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return copied, err
			}
			if err := copyFile(srcPath, dstPath); err != nil {
				return copied, err
			}
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

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// FileChange represents a changed config file
type FileChange struct {
	File     string
	Conflict bool // true if source also changed
}

// DetectChanges checks if config files or directories in worktree differ from source.
// Also detects conflicts where source changed too.
func (m *Manager) DetectChanges(wtPath string, files []string) ([]FileChange, error) {
	var changes []FileChange

	for _, file := range files {
		srcPath := filepath.Join(m.RepoRoot, file)
		dstPath := filepath.Join(wtPath, file)

		dstInfo, err := os.Stat(dstPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}

		if dstInfo.IsDir() {
			// For directories, walk and compare each file
			dirChanges, err := m.detectDirChanges(srcPath, dstPath, file)
			if err != nil {
				return nil, err
			}
			changes = append(changes, dirChanges...)
		} else {
			// Original file handling
			fileChange, hasChange, err := m.detectFileChange(srcPath, dstPath, file)
			if err != nil {
				return nil, err
			}
			if hasChange {
				changes = append(changes, fileChange)
			}
		}
	}

	return changes, nil
}

func (m *Manager) detectFileChange(srcPath, dstPath, file string) (FileChange, bool, error) {
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		return FileChange{}, false, err
	}

	srcContent, err := os.ReadFile(srcPath)
	if os.IsNotExist(err) {
		// File exists in worktree but not source - that's a change
		return FileChange{File: file, Conflict: false}, true, nil
	}
	if err != nil {
		return FileChange{}, false, err
	}

	// Compare contents
	if !bytes.Equal(srcContent, dstContent) {
		change := FileChange{File: file, Conflict: false}

		// Simple conflict detection by comparing mod times
		srcInfo, _ := os.Stat(srcPath)
		dstInfo, _ := os.Stat(dstPath)
		if srcInfo != nil && dstInfo != nil {
			if srcInfo.ModTime().After(dstInfo.ModTime()) {
				change.Conflict = true
			}
		}

		return change, true, nil
	}

	return FileChange{}, false, nil
}

func (m *Manager) detectDirChanges(srcDir, dstDir, baseFile string) ([]FileChange, error) {
	var changes []FileChange

	err := filepath.Walk(dstDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dstDir, path)
		if err != nil {
			return err
		}

		srcPath := filepath.Join(srcDir, relPath)
		file := filepath.Join(baseFile, relPath)

		change, hasChange, err := m.detectFileChange(srcPath, path, file)
		if err != nil {
			return err
		}
		if hasChange {
			changes = append(changes, change)
		}

		return nil
	})

	return changes, err
}

// MergeBack copies a file or directory from worktree back to source repo
func (m *Manager) MergeBack(wtPath, file string) error {
	srcPath := filepath.Join(wtPath, file)
	dstPath := filepath.Join(m.RepoRoot, file)

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return copyDir(srcPath, dstPath)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	return copyFile(srcPath, dstPath)
}

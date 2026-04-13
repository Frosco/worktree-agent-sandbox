package worktree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrWorktreeNotFound = errors.New("worktree does not exist")
var ErrBranchNotFound = errors.New("branch does not exist")

// Manager handles worktree operations for a repository.
type Manager struct {
	RepoRoot string
}

// NewManager creates a Manager for the repo at the given root.
func NewManager(repoRoot string) *Manager {
	return &Manager{
		RepoRoot: repoRoot,
	}
}

// WorktreePath returns the path where a worktree is located.
func (m *Manager) WorktreePath(name string) string {
	return filepath.Join(m.RepoRoot, ".claude", "worktrees", name)
}

// Exists checks if a worktree with the given name exists.
func (m *Manager) Exists(name string) bool {
	wtPath := m.WorktreePath(name)
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

// HasUncommittedChanges checks if a worktree has uncommitted changes.
// This includes untracked files, modified files, and staged changes.
func (m *Manager) HasUncommittedChanges(wtPath string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = wtPath
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// HasUnpushedCommits checks if a branch has commits not pushed to its upstream.
// Returns false if the branch has no upstream configured.
func (m *Manager) HasUnpushedCommits(branch string) bool {
	upstream := m.BranchUpstream(branch)
	if upstream == "" {
		return false
	}
	// Count commits in branch that are not in upstream
	cmd := exec.Command("git", "rev-list", "--count", upstream+".."+branch)
	cmd.Dir = m.RepoRoot
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	count := strings.TrimSpace(string(out))
	return count != "0"
}

// DeleteBranch deletes a local branch.
// If force is true, uses -D (force delete) which deletes even if not fully merged.
// Returns an error if the branch doesn't exist or can't be deleted.
func (m *Manager) DeleteBranch(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	cmd := exec.Command("git", "branch", flag, branch)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git branch %s: %w: %s", flag, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// WorktreeInfo holds information about a worktree.
type WorktreeInfo struct {
	Name   string // Directory name (e.g., "feature-auth")
	Branch string // Git branch (e.g., "worktree-feature-auth")
	Path   string // Full filesystem path
}

// List returns all worktrees in .claude/worktrees/ for this repo.
func (m *Manager) List() ([]WorktreeInfo, error) {
	wtDir := filepath.Join(m.RepoRoot, ".claude", "worktrees")

	entries, err := os.ReadDir(wtDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var worktrees []WorktreeInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		wtPath := filepath.Join(wtDir, name)
		branch := branchForWorktree(wtPath)
		worktrees = append(worktrees, WorktreeInfo{
			Name:   name,
			Branch: branch,
			Path:   wtPath,
		})
	}

	return worktrees, nil
}

// branchForWorktree reads the branch checked out in a worktree.
func branchForWorktree(wtPath string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = wtPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Remove removes a worktree by name.
// If force is true, removes even if worktree has uncommitted changes.
// Also deletes the associated local branch.
func (m *Manager) Remove(name string, force bool) error {
	wtPath := m.WorktreePath(name)

	if !m.Exists(name) {
		return ErrWorktreeNotFound
	}

	// Read the branch name before removing the worktree
	branch := branchForWorktree(wtPath)

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

	// Delete the local branch (force because remote may be gone)
	if branch != "" {
		_ = m.DeleteBranch(branch, true)
	}

	return nil
}

// FetchPrune fetches from origin and prunes stale remote-tracking refs.
func (m *Manager) FetchPrune() error {
	cmd := exec.Command("git", "fetch", "--prune", "origin")
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch --prune origin: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

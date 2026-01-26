package worktree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrWorktreeExists = errors.New("worktree already exists")

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

// Create creates a new worktree for the given branch.
// Creates the branch if it doesn't exist.
func (m *Manager) Create(branch string) (string, error) {
	wtPath := m.WorktreePath(branch)

	if m.Exists(branch) {
		return "", ErrWorktreeExists
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", err
	}

	// Check if branch exists
	checkCmd := exec.Command("git", "rev-parse", "--verify", branch)
	checkCmd.Dir = m.RepoRoot
	branchExists := checkCmd.Run() == nil

	var cmd *exec.Cmd
	if branchExists {
		cmd = exec.Command("git", "worktree", "add", wtPath, branch)
	} else {
		cmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	}
	cmd.Dir = m.RepoRoot

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return wtPath, nil
}

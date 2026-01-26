package worktree

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrNotGitRepo = errors.New("not a git repository")

// FindRepoRoot finds the root of the git repository containing dir.
// When called from a worktree, returns the main repository root, not the worktree path.
func FindRepoRoot(dir string) (string, error) {
	// Use --git-common-dir to get the main repo's .git directory,
	// which works correctly from both main checkout and worktrees
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", ErrNotGitRepo
	}

	gitDir := strings.TrimSpace(string(out))

	// gitDir is either ".git" (relative) or an absolute path like "/path/to/repo/.git"
	// For worktrees, it's always absolute. For main checkout, it may be relative.
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(dir, gitDir)
	}

	// Remove the /.git suffix to get the repo root
	repoRoot := filepath.Dir(gitDir)
	return repoRoot, nil
}

// GetRepoName extracts the repository name from its path.
func GetRepoName(repoRoot string) string {
	return filepath.Base(repoRoot)
}

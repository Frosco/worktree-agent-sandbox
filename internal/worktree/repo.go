package worktree

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrNotGitRepo = errors.New("not a git repository")

// FindRepoRoot finds the root of the git repository containing dir.
func FindRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", ErrNotGitRepo
	}
	return strings.TrimSpace(string(out)), nil
}

// GetRepoName extracts the repository name from its path.
func GetRepoName(repoRoot string) string {
	return filepath.Base(repoRoot)
}

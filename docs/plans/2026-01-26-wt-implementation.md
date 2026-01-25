# wt CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI that manages git worktrees with config file copying and Podman sandbox support for Claude Code.

**Architecture:** Cobra CLI with internal packages for config (TOML parsing), worktree (git operations), shell (init scripts), and sandbox (Podman). Shell function wrapper handles cd operations.

**Tech Stack:** Go, Cobra, go-toml/v2, Podman

---

## Task 1: Project Setup

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `cmd/wt/main.go`

**Step 1: Initialize Go module**

Run: `go mod init github.com/niref/wt`
Expected: Creates go.mod

**Step 2: Add dependencies**

Run: `go get github.com/spf13/cobra && go get github.com/pelletier/go-toml/v2`
Expected: Updates go.mod and creates go.sum

**Step 3: Create minimal main.go**

Create `cmd/wt/main.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wt-bin",
	Short: "Git worktree manager with Claude Code sandbox support",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 4: Verify it builds**

Run: `go build -o wt-bin ./cmd/wt && ./wt-bin --help`
Expected: Shows help with "Git worktree manager" description

**Step 5: Commit**

```bash
git add go.mod go.sum cmd/
git commit -m "feat: initialize Go project with Cobra CLI skeleton"
```

---

## Task 2: Config Package - Types and Loading

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing test for config loading**

Create `internal/config/config_test.go`:
```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlobalConfig(t *testing.T) {
	// Create temp dir for test config
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "wt")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "config.toml")
	configContent := `
copy_files = ["CLAUDE.md", ".envrc"]
extra_mounts = ["~/shared-libs", "~/data:ro"]
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadGlobalConfig(configPath)
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	if len(cfg.CopyFiles) != 2 {
		t.Errorf("expected 2 copy_files, got %d", len(cfg.CopyFiles))
	}
	if cfg.CopyFiles[0] != "CLAUDE.md" {
		t.Errorf("expected CLAUDE.md, got %s", cfg.CopyFiles[0])
	}
	if len(cfg.ExtraMounts) != 2 {
		t.Errorf("expected 2 extra_mounts, got %d", len(cfg.ExtraMounts))
	}
}

func TestLoadGlobalConfigMissing(t *testing.T) {
	cfg, err := LoadGlobalConfig("/nonexistent/config.toml")
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if cfg.CopyFiles != nil {
		t.Error("expected nil CopyFiles for missing config")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL - package not found

**Step 3: Write minimal implementation**

Create `internal/config/config.go`:
```go
package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Config represents wt configuration from TOML files
type Config struct {
	CopyFiles   []string `toml:"copy_files"`
	ExtraMounts []string `toml:"extra_mounts"`
}

// LoadGlobalConfig loads config from the given path.
// Returns empty config if file doesn't exist.
func LoadGlobalConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add Config type and LoadGlobalConfig"
```

---

## Task 3: Config Package - Repo Config and Merging

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing test for repo config and merging**

Add to `internal/config/config_test.go`:
```go
func TestLoadRepoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".wt.toml")
	configContent := `
copy_files = ["mise.local.toml"]
extra_mounts = ["~/project-data"]
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadRepoConfig failed: %v", err)
	}

	if len(cfg.CopyFiles) != 1 || cfg.CopyFiles[0] != "mise.local.toml" {
		t.Errorf("unexpected copy_files: %v", cfg.CopyFiles)
	}
}

func TestMergeConfigs(t *testing.T) {
	global := &Config{
		CopyFiles:   []string{"CLAUDE.md", ".envrc"},
		ExtraMounts: []string{"~/shared"},
	}
	repo := &Config{
		CopyFiles:   []string{"mise.local.toml"},
		ExtraMounts: []string{"~/project"},
	}

	merged := MergeConfigs(global, repo)

	expectedFiles := []string{"CLAUDE.md", ".envrc", "mise.local.toml"}
	if len(merged.CopyFiles) != 3 {
		t.Errorf("expected 3 copy_files, got %d: %v", len(merged.CopyFiles), merged.CopyFiles)
	}
	for i, f := range expectedFiles {
		if merged.CopyFiles[i] != f {
			t.Errorf("copy_files[%d]: expected %s, got %s", i, f, merged.CopyFiles[i])
		}
	}

	if len(merged.ExtraMounts) != 2 {
		t.Errorf("expected 2 extra_mounts, got %d", len(merged.ExtraMounts))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL - LoadRepoConfig and MergeConfigs undefined

**Step 3: Write implementation**

Add to `internal/config/config.go`:
```go
import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// LoadRepoConfig loads .wt.toml from the given repo root.
// Returns empty config if file doesn't exist.
func LoadRepoConfig(repoRoot string) (*Config, error) {
	path := filepath.Join(repoRoot, ".wt.toml")
	return LoadGlobalConfig(path)
}

// MergeConfigs combines global and repo configs.
// Repo config adds to global (does not replace).
func MergeConfigs(global, repo *Config) *Config {
	merged := &Config{}

	// Combine copy_files
	merged.CopyFiles = append(merged.CopyFiles, global.CopyFiles...)
	merged.CopyFiles = append(merged.CopyFiles, repo.CopyFiles...)

	// Combine extra_mounts
	merged.ExtraMounts = append(merged.ExtraMounts, global.ExtraMounts...)
	merged.ExtraMounts = append(merged.ExtraMounts, repo.ExtraMounts...)

	return merged
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add LoadRepoConfig and MergeConfigs"
```

---

## Task 4: Config Package - Default Paths Helper

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing test for default paths**

Add to `internal/config/config_test.go`:
```go
func TestDefaultPaths(t *testing.T) {
	// Override HOME for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", "/home/testuser")
	defer os.Setenv("HOME", origHome)

	paths := DefaultPaths()

	expectedConfig := "/home/testuser/.config/wt/config.toml"
	if paths.GlobalConfig != expectedConfig {
		t.Errorf("GlobalConfig: expected %s, got %s", expectedConfig, paths.GlobalConfig)
	}

	expectedWorktrees := "/home/testuser/.local/share/wt/worktrees"
	if paths.WorktreeBase != expectedWorktrees {
		t.Errorf("WorktreeBase: expected %s, got %s", expectedWorktrees, paths.WorktreeBase)
	}
}

func TestDefaultPathsWithXDG(t *testing.T) {
	origHome := os.Getenv("HOME")
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origDataHome := os.Getenv("XDG_DATA_HOME")

	os.Setenv("HOME", "/home/testuser")
	os.Setenv("XDG_CONFIG_HOME", "/custom/config")
	os.Setenv("XDG_DATA_HOME", "/custom/data")
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("XDG_CONFIG_HOME", origConfigHome)
		os.Setenv("XDG_DATA_HOME", origDataHome)
	}()

	paths := DefaultPaths()

	if paths.GlobalConfig != "/custom/config/wt/config.toml" {
		t.Errorf("GlobalConfig with XDG: expected /custom/config/wt/config.toml, got %s", paths.GlobalConfig)
	}
	if paths.WorktreeBase != "/custom/data/wt/worktrees" {
		t.Errorf("WorktreeBase with XDG: expected /custom/data/wt/worktrees, got %s", paths.WorktreeBase)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -v`
Expected: FAIL - DefaultPaths undefined

**Step 3: Write implementation**

Add to `internal/config/config.go`:
```go
// Paths holds default file/directory paths
type Paths struct {
	GlobalConfig string
	WorktreeBase string
}

// DefaultPaths returns XDG-compliant default paths
func DefaultPaths() Paths {
	home := os.Getenv("HOME")

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}

	return Paths{
		GlobalConfig: filepath.Join(configHome, "wt", "config.toml"),
		WorktreeBase: filepath.Join(dataHome, "wt", "worktrees"),
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add DefaultPaths with XDG support"
```

---

## Task 5: Worktree Package - Repo Detection

**Files:**
- Create: `internal/worktree/repo.go`
- Create: `internal/worktree/repo_test.go`

**Step 1: Write failing test for repo detection**

Create `internal/worktree/repo_test.go`:
```go
package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFindRepoRoot(t *testing.T) {
	// Create a temp git repo
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Test from repo root
	root, err := FindRepoRoot(repoDir)
	if err != nil {
		t.Fatalf("FindRepoRoot failed: %v", err)
	}
	if root != repoDir {
		t.Errorf("expected %s, got %s", repoDir, root)
	}

	// Test from subdirectory
	subDir := filepath.Join(repoDir, "src", "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	root, err = FindRepoRoot(subDir)
	if err != nil {
		t.Fatalf("FindRepoRoot from subdir failed: %v", err)
	}
	if root != repoDir {
		t.Errorf("expected %s, got %s", repoDir, root)
	}
}

func TestFindRepoRootNotGit(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := FindRepoRoot(tmpDir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestGetRepoName(t *testing.T) {
	name := GetRepoName("/home/user/dev/my-project")
	if name != "my-project" {
		t.Errorf("expected my-project, got %s", name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -v`
Expected: FAIL - package not found

**Step 3: Write implementation**

Create `internal/worktree/repo.go`:
```go
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/
git commit -m "feat(worktree): add FindRepoRoot and GetRepoName"
```

---

## Task 6: Worktree Package - Create Worktree

**Files:**
- Create: `internal/worktree/worktree.go`
- Modify: `internal/worktree/repo_test.go` → rename to `internal/worktree/worktree_test.go`

**Step 1: Write failing test for worktree creation**

Add to `internal/worktree/repo_test.go` (will be our main test file):
```go
func TestCreateWorktree(t *testing.T) {
	// Create a temp git repo with initial commit
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Initialize repo with a commit
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	// Create worktree
	wt := &Manager{
		RepoRoot:     repoDir,
		RepoName:     "myrepo",
		WorktreeBase: worktreeBase,
	}

	wtPath, err := wt.Create("feature-x")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	expectedPath := filepath.Join(worktreeBase, "myrepo", "feature-x")
	if wtPath != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, wtPath)
	}

	// Verify worktree exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory not created")
	}

	// Verify it's a git worktree
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = wtPath
	if err := cmd.Run(); err != nil {
		t.Error("created directory is not a git worktree")
	}
}

func TestCreateWorktreeAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	wt := &Manager{
		RepoRoot:     repoDir,
		RepoName:     "myrepo",
		WorktreeBase: worktreeBase,
	}

	// Create first time
	if _, err := wt.Create("feature-x"); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	// Second create should fail
	_, err := wt.Create("feature-x")
	if err == nil {
		t.Error("expected error for existing worktree")
	}
	if !errors.Is(err, ErrWorktreeExists) {
		t.Errorf("expected ErrWorktreeExists, got %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -v`
Expected: FAIL - Manager and Create undefined

**Step 3: Write implementation**

Create `internal/worktree/worktree.go`:
```go
package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
		return "", errors.New(string(out))
	}

	return wtPath, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/
git commit -m "feat(worktree): add Manager with Create method"
```

---

## Task 7: Worktree Package - List and Remove

**Files:**
- Modify: `internal/worktree/worktree.go`
- Modify: `internal/worktree/repo_test.go`

**Step 1: Write failing tests for List and Remove**

Add to test file:
```go
func TestListWorktrees(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	wt := &Manager{
		RepoRoot:     repoDir,
		RepoName:     "myrepo",
		WorktreeBase: worktreeBase,
	}

	// Create two worktrees
	wt.Create("feature-a")
	wt.Create("feature-b")

	list, err := wt.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should have 2 worktrees (not counting main)
	if len(list) != 2 {
		t.Errorf("expected 2 worktrees, got %d: %v", len(list), list)
	}
}

func TestRemoveWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	wt := &Manager{
		RepoRoot:     repoDir,
		RepoName:     "myrepo",
		WorktreeBase: worktreeBase,
	}

	// Create and remove
	wtPath, _ := wt.Create("feature-x")
	if err := wt.Remove("feature-x"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory still exists")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -v`
Expected: FAIL - List and Remove undefined

**Step 3: Write implementation**

Add to `internal/worktree/worktree.go`:
```go
import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
		return errors.New("worktree does not exist")
	}

	cmd := exec.Command("git", "worktree", "remove", wtPath)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(string(out))
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/
git commit -m "feat(worktree): add List and Remove methods"
```

---

## Task 8: Worktree Package - Copy Config Files

**Files:**
- Modify: `internal/worktree/worktree.go`
- Modify: `internal/worktree/repo_test.go`

**Step 1: Write failing test for file copying**

Add to test file:
```go
func TestCopyConfigFiles(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create source files in repo
	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("# Claude"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "mise.local.toml"), []byte("[tools]"), 0644); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	wt := &Manager{
		RepoRoot:     repoDir,
		RepoName:     "myrepo",
		WorktreeBase: worktreeBase,
	}

	wtPath, _ := wt.Create("feature-x")

	// Copy files
	filesToCopy := []string{"CLAUDE.md", "mise.local.toml", "nonexistent.txt"}
	copied, err := wt.CopyFiles(wtPath, filesToCopy)
	if err != nil {
		t.Fatalf("CopyFiles failed: %v", err)
	}

	// Should copy 2 files (skip nonexistent)
	if len(copied) != 2 {
		t.Errorf("expected 2 copied files, got %d", len(copied))
	}

	// Verify files exist in worktree
	content, err := os.ReadFile(filepath.Join(wtPath, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md not copied: %v", err)
	}
	if string(content) != "# Claude" {
		t.Errorf("content mismatch: %s", content)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -v`
Expected: FAIL - CopyFiles undefined

**Step 3: Write implementation**

Add to `internal/worktree/worktree.go`:
```go
import (
	"io"
	// ... existing imports
)

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

	_, err = io.Copy(dstFile, srcFile)
	return err
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/
git commit -m "feat(worktree): add CopyFiles method"
```

---

## Task 9: Worktree Package - Detect Config File Changes

**Files:**
- Modify: `internal/worktree/worktree.go`
- Modify: `internal/worktree/repo_test.go`

**Step 1: Write failing test for change detection**

Add to test file:
```go
func TestDetectConfigChanges(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create source files
	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "unchanged.txt"), []byte("same"), 0644); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	wt := &Manager{
		RepoRoot:     repoDir,
		RepoName:     "myrepo",
		WorktreeBase: worktreeBase,
	}

	wtPath, _ := wt.Create("feature-x")
	wt.CopyFiles(wtPath, []string{"CLAUDE.md", "unchanged.txt"})

	// Modify one file in worktree
	if err := os.WriteFile(filepath.Join(wtPath, "CLAUDE.md"), []byte("modified in worktree"), 0644); err != nil {
		t.Fatal(err)
	}

	changes, err := wt.DetectChanges(wtPath, []string{"CLAUDE.md", "unchanged.txt"})
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("expected 1 changed file, got %d", len(changes))
	}
	if len(changes) > 0 && changes[0].File != "CLAUDE.md" {
		t.Errorf("expected CLAUDE.md changed, got %s", changes[0].File)
	}
}

func TestDetectConflict(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	wt := &Manager{
		RepoRoot:     repoDir,
		RepoName:     "myrepo",
		WorktreeBase: worktreeBase,
	}

	wtPath, _ := wt.Create("feature-x")
	wt.CopyFiles(wtPath, []string{"CLAUDE.md"})

	// Modify in both places
	if err := os.WriteFile(filepath.Join(wtPath, "CLAUDE.md"), []byte("modified in worktree"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("modified in main"), 0644); err != nil {
		t.Fatal(err)
	}

	changes, _ := wt.DetectChanges(wtPath, []string{"CLAUDE.md"})
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if !changes[0].Conflict {
		t.Error("expected conflict=true")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -v`
Expected: FAIL - DetectChanges undefined

**Step 3: Write implementation**

Add to `internal/worktree/worktree.go`:
```go
import (
	"bytes"
	"crypto/sha256"
	// ... existing imports
)

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
			srcHash := sha256.Sum256(srcContent)
			dstHash := sha256.Sum256(dstContent)
			if srcHash != dstHash {
				// Check if source was modified after copy by comparing mod times
				srcInfo, _ := os.Stat(srcPath)
				dstInfo, _ := os.Stat(dstPath)
				if srcInfo != nil && dstInfo != nil {
					// If source is newer than destination, likely a conflict
					if srcInfo.ModTime().After(dstInfo.ModTime()) {
						change.Conflict = true
					}
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/
git commit -m "feat(worktree): add DetectChanges and MergeBack"
```

---

## Task 10: Shell Package - Init Script Generation

**Files:**
- Create: `internal/shell/shell.go`
- Create: `internal/shell/shell_test.go`

**Step 1: Write failing test for shell init**

Create `internal/shell/shell_test.go`:
```go
package shell

import (
	"strings"
	"testing"
)

func TestGenerateBashInit(t *testing.T) {
	script := GenerateInit("bash")

	// Should define wt function
	if !strings.Contains(script, "wt()") {
		t.Error("script should define wt function")
	}

	// Should call wt-bin
	if !strings.Contains(script, "wt-bin") {
		t.Error("script should call wt-bin")
	}

	// Should handle new and switch with cd
	if !strings.Contains(script, "new|switch") {
		t.Error("script should handle new and switch commands")
	}

	// Should use cd
	if !strings.Contains(script, "cd ") {
		t.Error("script should use cd for directory changes")
	}
}

func TestGenerateZshInit(t *testing.T) {
	script := GenerateInit("zsh")

	// zsh version should also work
	if !strings.Contains(script, "wt()") {
		t.Error("script should define wt function")
	}
}

func TestGenerateUnknownShell(t *testing.T) {
	script := GenerateInit("fish")

	// Should return empty or error message for unsupported shells
	if script != "" && !strings.Contains(script, "not supported") {
		t.Error("unsupported shell should return empty or error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/shell/... -v`
Expected: FAIL - package not found

**Step 3: Write implementation**

Create `internal/shell/shell.go`:
```go
package shell

// GenerateInit generates shell initialization script for the given shell
func GenerateInit(shell string) string {
	switch shell {
	case "bash", "zsh":
		return bashInit
	default:
		return "# Shell '" + shell + "' not supported. Use bash or zsh.\n"
	}
}

const bashInit = `# wt shell integration
# Add to your ~/.bashrc or ~/.zshrc:
#   eval "$(wt-bin shell-init bash)"

wt() {
    case "$1" in
        new|switch)
            local output
            output=$(wt-bin "$@" --print-path 2>&1)
            local exit_code=$?
            if [ $exit_code -eq 0 ] && [ -d "$output" ]; then
                cd "$output"
            else
                echo "$output" >&2
                return $exit_code
            fi
            ;;
        *)
            wt-bin "$@"
            ;;
    esac
}
`
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/shell/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/shell/
git commit -m "feat(shell): add shell init script generation"
```

---

## Task 11: CLI - Shell Init Command

**Files:**
- Create: `cmd/wt/shell_init.go`
- Create: `cmd/wt/shell_init_test.go`

**Step 1: Write failing test**

Create `cmd/wt/shell_init_test.go`:
```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestShellInitCommand(t *testing.T) {
	// Capture output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"shell-init", "bash"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("shell-init failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "wt()") {
		t.Error("output should contain wt function")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/wt/... -v`
Expected: FAIL - shell-init command not found

**Step 3: Write implementation**

Create `cmd/wt/shell_init.go`:
```go
package main

import (
	"fmt"

	"github.com/niref/wt/internal/shell"
	"github.com/spf13/cobra"
)

var shellInitCmd = &cobra.Command{
	Use:   "shell-init [bash|zsh]",
	Short: "Output shell initialization script",
	Long:  `Output shell function for directory-changing commands. Add to your shell rc file.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		script := shell.GenerateInit(args[0])
		fmt.Fprint(cmd.OutOrStdout(), script)
	},
}

func init() {
	rootCmd.AddCommand(shellInitCmd)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/wt/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/wt/
git commit -m "feat(cli): add shell-init command"
```

---

## Task 12: CLI - New Command

**Files:**
- Create: `cmd/wt/new.go`
- Create: `cmd/wt/new_test.go`

**Step 1: Write failing test**

Create `cmd/wt/new_test.go`:
```go
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) (repoDir, worktreeBase string) {
	tmpDir := t.TempDir()
	repoDir = filepath.Join(tmpDir, "myrepo")
	worktreeBase = filepath.Join(tmpDir, "worktrees")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	return repoDir, worktreeBase
}

func TestNewCommand(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	// Create a CLAUDE.md to copy
	os.WriteFile(filepath.Join(repoDir, "CLAUDE.md"), []byte("# Claude"), 0644)

	// Create global config
	configDir := filepath.Join(t.TempDir(), "config", "wt")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.toml")
	os.WriteFile(configPath, []byte(`copy_files = ["CLAUDE.md"]`), 0644)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Override paths for test
	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	rootCmd.SetArgs([]string{"new", "feature-test",
		"--worktree-base", worktreeBase,
		"--config", configPath,
		"--print-path",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("new command failed: %v\n%s", err, buf.String())
	}

	output := strings.TrimSpace(buf.String())
	expectedPath := filepath.Join(worktreeBase, "myrepo", "feature-test")

	if output != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, output)
	}

	// Verify worktree was created
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("worktree not created")
	}

	// Verify CLAUDE.md was copied
	if _, err := os.Stat(filepath.Join(expectedPath, "CLAUDE.md")); os.IsNotExist(err) {
		t.Error("CLAUDE.md not copied")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/wt/... -v`
Expected: FAIL - new command not found

**Step 3: Write implementation**

Create `cmd/wt/new.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	newPrintPath    bool
	newWorktreeBase string
	newConfigPath   string
)

var newCmd = &cobra.Command{
	Use:   "new <branch>",
	Short: "Create a new worktree for a branch",
	Long:  `Creates a worktree and copies config files. Creates branch if it doesn't exist.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]

		// Find repo root from current directory
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		// Determine paths
		paths := config.DefaultPaths()
		worktreeBase := newWorktreeBase
		if worktreeBase == "" {
			worktreeBase = paths.WorktreeBase
		}
		configPath := newConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		// Load configs
		globalCfg, err := config.LoadGlobalConfig(configPath)
		if err != nil {
			return fmt.Errorf("loading global config: %w", err)
		}
		repoCfg, err := config.LoadRepoConfig(repoRoot)
		if err != nil {
			return fmt.Errorf("loading repo config: %w", err)
		}
		cfg := config.MergeConfigs(globalCfg, repoCfg)

		// Create worktree
		mgr := worktree.NewManager(repoRoot, worktreeBase)
		wtPath, err := mgr.Create(branch)
		if err != nil {
			if err == worktree.ErrWorktreeExists {
				return fmt.Errorf("worktree already exists, use 'wt switch %s' instead", branch)
			}
			return err
		}

		// Copy config files
		if len(cfg.CopyFiles) > 0 {
			copied, err := mgr.CopyFiles(wtPath, cfg.CopyFiles)
			if err != nil {
				return fmt.Errorf("copying files: %w", err)
			}
			if !newPrintPath && len(copied) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Copied: %v\n", copied)
			}
		}

		if newPrintPath {
			fmt.Fprintln(cmd.OutOrStdout(), wtPath)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Created worktree at %s\n", wtPath)
		}

		return nil
	},
}

func init() {
	newCmd.Flags().BoolVar(&newPrintPath, "print-path", false, "Only print the worktree path (for shell integration)")
	newCmd.Flags().StringVar(&newWorktreeBase, "worktree-base", "", "Override worktree base directory")
	newCmd.Flags().StringVar(&newConfigPath, "config", "", "Override global config path")
	rootCmd.AddCommand(newCmd)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/wt/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/wt/
git commit -m "feat(cli): add new command"
```

---

## Task 13: CLI - Switch Command

**Files:**
- Create: `cmd/wt/switch.go`

**Step 1: Write failing test**

Add to `cmd/wt/new_test.go`:
```go
func TestSwitchCommand(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	buf := new(bytes.Buffer)

	// First switch creates the worktree
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"switch", "feature-switch",
		"--worktree-base", worktreeBase,
		"--print-path",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("first switch failed: %v", err)
	}

	expectedPath := filepath.Join(worktreeBase, "myrepo", "feature-switch")
	if strings.TrimSpace(buf.String()) != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, buf.String())
	}

	// Second switch should work (idempotent)
	buf.Reset()
	rootCmd.SetArgs([]string{"switch", "feature-switch",
		"--worktree-base", worktreeBase,
		"--print-path",
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("second switch failed: %v", err)
	}

	if strings.TrimSpace(buf.String()) != expectedPath {
		t.Errorf("second switch: expected %s, got %s", expectedPath, buf.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/wt/... -v`
Expected: FAIL - switch command not found

**Step 3: Write implementation**

Create `cmd/wt/switch.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	switchPrintPath    bool
	switchWorktreeBase string
	switchConfigPath   string
)

var switchCmd = &cobra.Command{
	Use:   "switch <branch>",
	Short: "Switch to a worktree (create if needed)",
	Long:  `Switch to an existing worktree or create one if it doesn't exist.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		paths := config.DefaultPaths()
		worktreeBase := switchWorktreeBase
		if worktreeBase == "" {
			worktreeBase = paths.WorktreeBase
		}
		configPath := switchConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		mgr := worktree.NewManager(repoRoot, worktreeBase)

		// If worktree exists, just return its path
		if mgr.Exists(branch) {
			wtPath := mgr.WorktreePath(branch)
			if switchPrintPath {
				fmt.Fprintln(cmd.OutOrStdout(), wtPath)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Switched to %s\n", wtPath)
			}
			return nil
		}

		// Otherwise, create it (same logic as new)
		globalCfg, err := config.LoadGlobalConfig(configPath)
		if err != nil {
			return fmt.Errorf("loading global config: %w", err)
		}
		repoCfg, err := config.LoadRepoConfig(repoRoot)
		if err != nil {
			return fmt.Errorf("loading repo config: %w", err)
		}
		cfg := config.MergeConfigs(globalCfg, repoCfg)

		wtPath, err := mgr.Create(branch)
		if err != nil {
			return err
		}

		if len(cfg.CopyFiles) > 0 {
			copied, err := mgr.CopyFiles(wtPath, cfg.CopyFiles)
			if err != nil {
				return fmt.Errorf("copying files: %w", err)
			}
			if !switchPrintPath && len(copied) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Copied: %v\n", copied)
			}
		}

		if switchPrintPath {
			fmt.Fprintln(cmd.OutOrStdout(), wtPath)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Created and switched to %s\n", wtPath)
		}

		return nil
	},
}

func init() {
	switchCmd.Flags().BoolVar(&switchPrintPath, "print-path", false, "Only print the worktree path")
	switchCmd.Flags().StringVar(&switchWorktreeBase, "worktree-base", "", "Override worktree base directory")
	switchCmd.Flags().StringVar(&switchConfigPath, "config", "", "Override global config path")
	rootCmd.AddCommand(switchCmd)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/wt/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/wt/
git commit -m "feat(cli): add switch command"
```

---

## Task 14: CLI - List Command

**Files:**
- Create: `cmd/wt/list.go`

**Step 1: Write failing test**

Add to test file:
```go
func TestListCommand(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create some worktrees first
	mgr := worktree.NewManager(repoDir, worktreeBase)
	mgr.Create("feature-a")
	mgr.Create("feature-b")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"list", "--worktree-base", worktreeBase})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "feature-a") {
		t.Error("output should contain feature-a")
	}
	if !strings.Contains(output, "feature-b") {
		t.Error("output should contain feature-b")
	}
}
```

Also add import at top:
```go
import (
	"github.com/niref/wt/internal/worktree"
)
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/wt/... -v`
Expected: FAIL - list command not found

**Step 3: Write implementation**

Create `cmd/wt/list.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var listWorktreeBase string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List worktrees for current repo",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		paths := config.DefaultPaths()
		worktreeBase := listWorktreeBase
		if worktreeBase == "" {
			worktreeBase = paths.WorktreeBase
		}

		mgr := worktree.NewManager(repoRoot, worktreeBase)
		worktrees, err := mgr.List()
		if err != nil {
			return err
		}

		if len(worktrees) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found")
			return nil
		}

		for _, wt := range worktrees {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", wt.Branch, wt.Path)
		}

		return nil
	},
}

func init() {
	listCmd.Flags().StringVar(&listWorktreeBase, "worktree-base", "", "Override worktree base directory")
	rootCmd.AddCommand(listCmd)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/wt/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/wt/
git commit -m "feat(cli): add list command"
```

---

## Task 15: CLI - Remove Command

**Files:**
- Create: `cmd/wt/remove.go`

**Step 1: Write failing test**

Add to test file:
```go
func TestRemoveCommand(t *testing.T) {
	repoDir, worktreeBase := setupTestRepo(t)

	origDir, _ := os.Getwd()
	os.Chdir(repoDir)
	defer os.Chdir(origDir)

	// Create a worktree
	mgr := worktree.NewManager(repoDir, worktreeBase)
	wtPath, _ := mgr.Create("feature-remove")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"remove", "feature-remove",
		"--worktree-base", worktreeBase,
		"--force", // Skip change detection prompt
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// Verify worktree is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree should be removed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/wt/... -v`
Expected: FAIL - remove command not found

**Step 3: Write implementation**

Create `cmd/wt/remove.go`:
```go
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	removeWorktreeBase string
	removeConfigPath   string
	removeForce        bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <branch>",
	Short: "Remove a worktree",
	Long:  `Remove a worktree. Detects config file changes and prompts for action.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		paths := config.DefaultPaths()
		worktreeBase := removeWorktreeBase
		if worktreeBase == "" {
			worktreeBase = paths.WorktreeBase
		}
		configPath := removeConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		mgr := worktree.NewManager(repoRoot, worktreeBase)

		if !mgr.Exists(branch) {
			return fmt.Errorf("worktree '%s' does not exist", branch)
		}

		wtPath := mgr.WorktreePath(branch)

		// Check for config file changes (unless --force)
		if !removeForce {
			globalCfg, _ := config.LoadGlobalConfig(configPath)
			repoCfg, _ := config.LoadRepoConfig(repoRoot)
			cfg := config.MergeConfigs(globalCfg, repoCfg)

			if len(cfg.CopyFiles) > 0 {
				changes, err := mgr.DetectChanges(wtPath, cfg.CopyFiles)
				if err != nil {
					return fmt.Errorf("detecting changes: %w", err)
				}

				if len(changes) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "These files were modified:")
					for _, c := range changes {
						conflict := ""
						if c.Conflict {
							conflict = " (CONFLICT: source also changed)"
						}
						fmt.Fprintf(cmd.OutOrStdout(), "  %s%s\n", c.File, conflict)
					}
					fmt.Fprintln(cmd.OutOrStdout())
					fmt.Fprintln(cmd.OutOrStdout(), "[m] Merge back to main worktree")
					fmt.Fprintln(cmd.OutOrStdout(), "[k] Keep original (discard changes)")
					fmt.Fprintln(cmd.OutOrStdout(), "[a] Abort remove")
					fmt.Fprint(cmd.OutOrStdout(), "Choice: ")

					reader := bufio.NewReader(os.Stdin)
					input, _ := reader.ReadString('\n')
					input = strings.TrimSpace(strings.ToLower(input))

					switch input {
					case "m":
						for _, c := range changes {
							if c.Conflict {
								fmt.Fprintf(cmd.ErrOrStderr(), "Skipping %s due to conflict\n", c.File)
								continue
							}
							if err := mgr.MergeBack(wtPath, c.File); err != nil {
								fmt.Fprintf(cmd.ErrOrStderr(), "Failed to merge %s: %v\n", c.File, err)
							} else {
								fmt.Fprintf(cmd.OutOrStdout(), "Merged %s\n", c.File)
							}
						}
					case "k":
						// Continue with removal
					case "a":
						return fmt.Errorf("aborted")
					default:
						return fmt.Errorf("invalid choice")
					}
				}
			}
		}

		if err := mgr.Remove(branch); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree '%s'\n", branch)
		return nil
	},
}

func init() {
	removeCmd.Flags().StringVar(&removeWorktreeBase, "worktree-base", "", "Override worktree base directory")
	removeCmd.Flags().StringVar(&removeConfigPath, "config", "", "Override global config path")
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Skip change detection")
	rootCmd.AddCommand(removeCmd)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/wt/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/wt/
git commit -m "feat(cli): add remove command with change detection"
```

---

## Task 16: Sandbox Package - Containerfile

**Files:**
- Create: `Containerfile`

**Step 1: Create Containerfile**

Create `Containerfile`:
```dockerfile
FROM node:20-bookworm-slim

# Install basic tools
RUN apt-get update && apt-get install -y \
    git \
    curl \
    ripgrep \
    fd-find \
    && rm -rf /var/lib/apt/lists/*

# Install mise
RUN curl https://mise.run | sh
ENV PATH="/root/.local/bin:$PATH"

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user matching typical host UID
ARG USER_ID=1000
ARG GROUP_ID=1000
RUN groupadd -g $GROUP_ID appuser && \
    useradd -m -u $USER_ID -g $GROUP_ID appuser

USER appuser
WORKDIR /home/appuser

# Mise for non-root user
RUN curl https://mise.run | sh
ENV PATH="/home/appuser/.local/bin:$PATH"

# Entry script will be bind-mounted
CMD ["bash"]
```

**Step 2: Verify it builds**

Run: `podman build -t wt-sandbox -f Containerfile .`
Expected: Build succeeds (may take a few minutes)

**Step 3: Commit**

```bash
git add Containerfile
git commit -m "feat(sandbox): add Containerfile for Claude Code sandbox"
```

---

## Task 17: Sandbox Package - Container Management

**Files:**
- Create: `internal/sandbox/sandbox.go`
- Create: `internal/sandbox/sandbox_test.go`

**Step 1: Write failing test**

Create `internal/sandbox/sandbox_test.go`:
```go
package sandbox

import (
	"os/exec"
	"strings"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	opts := &Options{
		WorktreePath:   "/home/user/worktrees/myrepo/feature",
		MainGitDir:     "/home/user/dev/myrepo/.git",
		ClaudeDir:      "/home/user/.claude",
		ExtraMounts:    []string{"/home/user/shared", "/data:ro"},
		ContainerImage: "wt-sandbox",
	}

	args := opts.BuildArgs()

	// Should contain userns keep-id
	found := false
	for _, arg := range args {
		if arg == "--userns=keep-id" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing --userns=keep-id")
	}

	// Check volume mounts
	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "-v /home/user/worktrees/myrepo/feature:/home/user/worktrees/myrepo/feature:Z") {
		t.Error("missing worktree mount")
	}
	if !strings.Contains(argStr, "-v /home/user/.claude:/home/user/.claude:ro") {
		t.Error("missing claude dir mount")
	}
}

func TestPodmanAvailable(t *testing.T) {
	err := CheckPodmanAvailable()
	// This test depends on podman being installed
	if err != nil {
		t.Skipf("podman not available: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/sandbox/... -v`
Expected: FAIL - package not found

**Step 3: Write implementation**

Create `internal/sandbox/sandbox.go`:
```go
package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options configures the sandbox container
type Options struct {
	WorktreePath   string
	MainGitDir     string
	ClaudeDir      string
	ExtraMounts    []string
	ContainerImage string
	RunMiseInstall bool
	StartClaude    bool
}

// CheckPodmanAvailable verifies podman is installed
func CheckPodmanAvailable() error {
	cmd := exec.Command("podman", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman not found. Install with: sudo pacman -S podman")
	}
	return nil
}

// BuildArgs constructs podman run arguments
func (o *Options) BuildArgs() []string {
	args := []string{
		"run",
		"--rm",
		"-it",
		"--userns=keep-id",
		"--dns=8.8.8.8",
	}

	// Mount worktree at same path
	args = append(args, "-v", fmt.Sprintf("%s:%s:Z", o.WorktreePath, o.WorktreePath))

	// Mount main git dir read-only
	if o.MainGitDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s:ro", o.MainGitDir, o.MainGitDir))
	}

	// Mount claude dir read-only
	if o.ClaudeDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s:ro", o.ClaudeDir, o.ClaudeDir))
	}

	// Extra mounts
	for _, mount := range o.ExtraMounts {
		path := mount
		mode := "Z"
		if strings.HasSuffix(mount, ":ro") {
			path = strings.TrimSuffix(mount, ":ro")
			mode = "ro"
		}
		// Expand ~ to home directory
		if strings.HasPrefix(path, "~/") {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, path[2:])
		}
		args = append(args, "-v", fmt.Sprintf("%s:%s:%s", path, path, mode))
	}

	// Working directory
	args = append(args, "-w", o.WorktreePath)

	// Image
	args = append(args, o.ContainerImage)

	return args
}

// BuildImage builds the sandbox container image
func BuildImage(containerfilePath, imageName string) error {
	cmd := exec.Command("podman", "build", "-t", imageName, "-f", containerfilePath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ImageExists checks if an image exists locally
func ImageExists(imageName string) bool {
	cmd := exec.Command("podman", "image", "exists", imageName)
	return cmd.Run() == nil
}

// Run starts the sandbox container
func Run(opts *Options) error {
	args := opts.BuildArgs()

	// Build the command to run inside
	var innerCmd string
	if opts.RunMiseInstall && opts.StartClaude {
		innerCmd = "mise install && claude --dangerously-skip-permissions"
	} else if opts.RunMiseInstall {
		innerCmd = "mise install && bash"
	} else if opts.StartClaude {
		innerCmd = "claude --dangerously-skip-permissions"
	} else {
		innerCmd = "bash"
	}

	args = append(args, "bash", "-c", innerCmd)

	cmd := exec.Command("podman", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/sandbox/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/sandbox/
git commit -m "feat(sandbox): add container management package"
```

---

## Task 18: CLI - Sandbox Command

**Files:**
- Create: `cmd/wt/sandbox.go`

**Step 1: Write implementation** (This command is hard to unit test due to interactive container)

Create `cmd/wt/sandbox.go`:
```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/sandbox"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	sandboxMounts       []string
	sandboxWorktreeBase string
	sandboxConfigPath   string
	sandboxNoClaude     bool
	sandboxNoMise       bool
	sandboxImage        string
)

var sandboxCmd = &cobra.Command{
	Use:   "sandbox [branch]",
	Short: "Run Claude Code in a sandboxed container",
	Long:  `Start a Podman container with the worktree mounted and run Claude with --dangerously-skip-permissions.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check podman is available
		if err := sandbox.CheckPodmanAvailable(); err != nil {
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		paths := config.DefaultPaths()
		worktreeBase := sandboxWorktreeBase
		if worktreeBase == "" {
			worktreeBase = paths.WorktreeBase
		}
		configPath := sandboxConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		// Load config for extra mounts
		globalCfg, _ := config.LoadGlobalConfig(configPath)
		repoCfg, _ := config.LoadRepoConfig(repoRoot)
		cfg := config.MergeConfigs(globalCfg, repoCfg)

		mgr := worktree.NewManager(repoRoot, worktreeBase)

		var wtPath string

		if len(args) > 0 {
			branch := args[0]
			// Switch to (or create) worktree
			if mgr.Exists(branch) {
				wtPath = mgr.WorktreePath(branch)
			} else {
				var err error
				wtPath, err = mgr.Create(branch)
				if err != nil {
					return err
				}
				// Copy config files
				if len(cfg.CopyFiles) > 0 {
					mgr.CopyFiles(wtPath, cfg.CopyFiles)
				}
			}
		} else {
			// Use current directory if it's a worktree managed by us
			wtPath = cwd
		}

		// Find the main .git directory
		mainGitDir := filepath.Join(repoRoot, ".git")

		// Claude credentials directory
		home, _ := os.UserHomeDir()
		claudeDir := filepath.Join(home, ".claude")

		// Combine extra mounts from config and flags
		allMounts := append(cfg.ExtraMounts, sandboxMounts...)

		// Build/check image
		imageName := sandboxImage
		if imageName == "" {
			imageName = "wt-sandbox"
		}

		if !sandbox.ImageExists(imageName) {
			fmt.Fprintln(cmd.OutOrStdout(), "Building sandbox image (this may take a few minutes)...")
			// Look for Containerfile in repo root or installed location
			containerfile := filepath.Join(repoRoot, "Containerfile")
			if _, err := os.Stat(containerfile); os.IsNotExist(err) {
				return fmt.Errorf("Containerfile not found. Run from wt repo or specify --image")
			}
			if err := sandbox.BuildImage(containerfile, imageName); err != nil {
				return fmt.Errorf("building image: %w", err)
			}
		}

		opts := &sandbox.Options{
			WorktreePath:   wtPath,
			MainGitDir:     mainGitDir,
			ClaudeDir:      claudeDir,
			ExtraMounts:    allMounts,
			ContainerImage: imageName,
			RunMiseInstall: !sandboxNoMise,
			StartClaude:    !sandboxNoClaude,
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Starting sandbox in %s...\n", wtPath)
		return sandbox.Run(opts)
	},
}

func init() {
	sandboxCmd.Flags().StringArrayVarP(&sandboxMounts, "mount", "m", nil, "Additional paths to mount")
	sandboxCmd.Flags().StringVar(&sandboxWorktreeBase, "worktree-base", "", "Override worktree base directory")
	sandboxCmd.Flags().StringVar(&sandboxConfigPath, "config", "", "Override global config path")
	sandboxCmd.Flags().BoolVar(&sandboxNoClaude, "no-claude", false, "Don't start Claude, just get a shell")
	sandboxCmd.Flags().BoolVar(&sandboxNoMise, "no-mise", false, "Don't run mise install")
	sandboxCmd.Flags().StringVar(&sandboxImage, "image", "", "Container image to use")
	rootCmd.AddCommand(sandboxCmd)
}
```

**Step 2: Verify it compiles**

Run: `go build ./cmd/wt/...`
Expected: Compiles successfully

**Step 3: Commit**

```bash
git add cmd/wt/
git commit -m "feat(cli): add sandbox command"
```

---

## Task 19: Final Integration - Build and Test

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 2: Build final binary**

Run: `go build -o wt-bin ./cmd/wt`
Expected: Binary created

**Step 3: Manual smoke test**

```bash
# Test shell-init
./wt-bin shell-init bash

# In a git repo, test new
./wt-bin new test-branch --print-path

# Test list
./wt-bin list

# Test remove
./wt-bin remove test-branch --force
```

**Step 4: Commit any fixes and tag**

```bash
git add -A
git commit -m "chore: finalize MVP implementation"
git tag v0.1.0
```

---

## Summary

The implementation is complete when:

1. All tests pass: `go test ./... -v`
2. Binary builds: `go build -o wt-bin ./cmd/wt`
3. Commands work:
   - `wt-bin shell-init bash` - outputs shell function
   - `wt-bin new <branch>` - creates worktree
   - `wt-bin switch <branch>` - switches/creates worktree
   - `wt-bin list` - lists worktrees
   - `wt-bin remove <branch>` - removes with change detection
   - `wt-bin sandbox [branch]` - runs Claude in container

# wt v2: Worktree Navigator & Cleaner

## Context

Claude Code now has native worktree support (`claude --worktree`) that covers creation, gitignored file copying (`.worktreeinclude`), and cross-worktree memory sharing. This makes most of wt's original functionality redundant. This spec describes slimming wt down to the features Claude Code doesn't cover: navigating between worktrees and batch cleanup.

## Scope

### In scope
- Rework `wt switch`, `wt list`, `wt prune`, and shell integration to use Claude Code's worktree location (`.claude/worktrees/`)
- Delete creation, config sync, memory sync, and merge-back features
- Fix prune to actually force-delete branches after user confirmation

### Out of scope
- `wt sandbox` (Podman container support) — unchanged
- Any new features

## Prerequisites (per repo, one-time)

- Add `.worktreeinclude` to repo root listing gitignored files to copy (e.g., `CLAUDE.md`)
- Ensure `.claude/worktrees/` is in `.gitignore`

## Commands

### `wt switch [name]`

Navigate to an existing worktree.

- **No argument**: interactive picker (charmbracelet/huh) listing all worktrees in `.claude/worktrees/`
- **With argument**: outputs the worktree path for the shell wrapper to `cd` into
- Display uses the **directory name** (what you pass to `claude --worktree`), not the git branch name

### `wt list`

List worktrees for the current repo.

- Reads directories from `.claude/worktrees/` in the current repo
- Displays worktree name and branch name
- Output format: `<name>\t<branch>\t<path>`

### `wt prune`

Batch cleanup of worktrees.

- Lists all worktrees in `.claude/worktrees/`
- For each worktree, checks for uncommitted changes and unpushed commits
- Warns the user about any issues found
- On user confirmation, **force-deletes** the worktree and its branch (fixes current behavior where branches with changes aren't actually deleted after confirmation)
- Removal steps: `git worktree remove --force <path>`, then `git branch -D <branch>`
- `--dry-run` flag to preview without changes

### `wt shell-init`

Generates shell functions for bash/zsh.

- Wraps `wt switch` to capture output path and `cd` into worktree
- Drops the `wt new` wrapper (command no longer exists)

### `wt sandbox`

Unchanged. Podman container orchestration stays as-is.

## Architecture Changes

### Manager struct

```go
type Manager struct {
    RepoRoot string
}

func NewManager(repoRoot string) *Manager
```

Worktree base path is derived as `<RepoRoot>/.claude/worktrees/` — no longer configurable or stored in the struct.

### Remaining Manager methods

| Method | Purpose |
|---|---|
| `WorktreePath(name)` | Returns `.claude/worktrees/<name>/` |
| `Exists(name)` | Checks if worktree directory exists |
| `List()` | Lists worktree directories |
| `Remove(name, force)` | Removes worktree + branch (branch name read from `git worktree list`, not assumed) |
| `BranchExists(branch)` | Checks local branch existence |
| `HasUncommittedChanges(path)` | Checks for dirty working tree |
| `HasUnpushedCommits(branch)` | Checks for unpushed work |
| `DeleteBranch(branch, force)` | Deletes local branch |
| `FetchPrune()` | Fetches and prunes stale remote refs |

### Naming convention

Claude Code creates worktrees with directory name `<name>` and branch `worktree-<name>`. The `wt` tool uses the directory name as the primary identifier in all user-facing output. Branch names are an implementation detail shown only in `wt list`.

### WorktreeInfo struct

```go
type WorktreeInfo struct {
    Name   string // Directory name (e.g., "feature-auth")
    Branch string // Git branch (e.g., "worktree-feature-auth")
    Path   string // Full filesystem path
}
```

Changed from `Branch`-primary to `Name`-primary to match Claude Code's convention.

## Deletions

### Files to delete
- `cmd/wt/new.go` and `cmd/wt/new_test.go`
- `cmd/wt/remove.go`
- `cmd/wt/config_changes.go`
- `internal/config/config.go` and `internal/config/config_test.go`
- `internal/worktree/memory.go` and `internal/worktree/memory_test.go`

### Functions/types to remove from `worktree.go`
- `Create`, `CopyFiles`, `SaveSnapshot`, `RemoveSnapshot`
- `DetectChanges`, `detectFileChange`, `detectDirChanges`
- `MergeBack`, `mergeThreeWay`
- `copyFile`, `copyDir`
- `FileChange`, `MergeStatus`, `MergeResult` types
- `SnapshotPath` method

### Dependencies to remove
- `github.com/pelletier/go-toml/v2` (was only used by config package)

## Testing

- Tests use real temp directories (no mocking), unchanged approach
- Memory-related test isolation (`t.Setenv("HOME", ...)`) no longer needed
- Tests for deleted commands are deleted with their commands
- Prune tests updated to verify force-delete behavior
- Switch/list tests updated to use `.claude/worktrees/` paths

# Design: `wt prune` Command

## Overview

`wt prune` removes worktrees whose branches have been deleted from the remote (merged or manually deleted). It also deletes the local branch since it's no longer needed.

**Safety guarantees:**
- Only considers branches with upstream tracking configured (never touches local-only work)
- Prompts for worktrees with uncommitted changes or unpushed commits
- Prompts for config file changes (same as `wt remove`)
- Auto-fetches to ensure accurate remote state

## Algorithm

1. Run `git fetch --prune origin` to update remote refs
2. Get list of worktrees via `mgr.List()`
3. For each worktree branch:
   a. Check if branch has upstream tracking configured
      - If no upstream → skip (never pushed)
   b. Check if upstream remote ref still exists
      - If exists → skip (not merged/deleted)
   c. Branch is a prune candidate
4. If no candidates: print "Nothing to prune" and exit
5. For each candidate:
   a. Check for unpushed commits (local ahead of merge-base)
   b. Check for uncommitted changes in worktree
   c. If either: prompt "Remove <branch>? It has <issue> [y/n]"
   d. Run config change detection (same as `wt remove`)
   e. Remove worktree via `mgr.Remove()`
   f. Delete local branch via `git branch -d <branch>`
6. Print summary with branch names:
   ```
   Pruned 3 worktrees:
     - feature-auth
     - fix-typo
     - refactor-config
   ```

## CLI Interface

```
wt prune

Flags:
  --force, -f       Force removal even if worktrees have uncommitted changes
  --skip-changes    Skip config file change detection
  --no-fetch        Skip git fetch --prune (use current remote refs)
  --dry-run, -n     Show what would be pruned without doing it
```

The `--force` and `--skip-changes` flags mirror `wt remove` for consistency.

## Implementation Changes

### New methods in `internal/worktree/manager.go`

```go
// BranchUpstream returns the upstream ref for a branch, or empty string if none
func (m *Manager) BranchUpstream(branch string) string

// DeleteBranch deletes a local branch
func (m *Manager) DeleteBranch(branch string) error

// HasUnpushedCommits checks if branch has commits not on its upstream
func (m *Manager) HasUnpushedCommits(branch string) bool

// HasUncommittedChanges checks if worktree has uncommitted changes
func (m *Manager) HasUncommittedChanges(wtPath string) bool
```

### New file `cmd/wt/prune.go`

- Cobra command with flags
- Orchestrates the prune algorithm
- Reuses config change detection logic from `remove.go` (may need to extract shared helper)

## Error Handling & Edge Cases

| Scenario | Behavior |
|----------|----------|
| No worktrees exist | "No worktrees found" (same as `wt list`) |
| All worktrees are local-only | "Nothing to prune (no branches with remote tracking)" |
| All tracked branches still on remote | "Nothing to prune" |
| `git fetch` fails (network error) | Error out with message, suggest `--no-fetch` |
| Worktree removal fails | Report error, continue with remaining worktrees |
| Branch delete fails after worktree removed | Report error, continue (worktree already gone) |
| User declines prompted removal | Skip that worktree, continue with others |
| User aborts during config change prompt | Stop processing, report what was already pruned |

## Testing Strategy

### Unit tests for new Manager methods

- `BranchUpstream` - with/without upstream configured
- `HasUnpushedCommits` - ahead, even, no upstream
- `HasUncommittedChanges` - clean, modified, staged, untracked

### Integration tests for prune command

- Setup: create worktrees with various states (local-only, tracked+gone, tracked+exists)
- Verify only correct branches are pruned
- Verify prompts trigger for uncommitted changes
- Test `--dry-run`, `--force`, `--no-fetch` flags

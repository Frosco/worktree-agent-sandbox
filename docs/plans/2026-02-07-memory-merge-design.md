# Design: MEMORY.md Merge Across Worktrees

## Problem

Claude Code stores project memory in `~/.claude/projects/<encoded-path>/memory/`,
keyed by filesystem path. When `wt` creates worktrees at different paths, each gets
isolated memory. Insights from one branch are invisible to others and lost when the
worktree is removed.

## Solution

Extend the existing config file merge-back mechanism to handle Claude Code's memory
directory. Memory is copied on worktree creation and merged back on removal, using the
same three-way merge infrastructure already in place for config files.

## Path Encoding

Claude Code encodes project paths as directory names:

1. Strip leading `/`
2. Replace all `/` and `.` with `-`
3. Prepend `-`

Example:
```
/home/user/dev/my-project  →  -home-user-dev-my-project
/home/user/.local/share/wt/worktrees/my-project/feat-x
    →  -home-user--local-share-wt-worktrees-my-project-feat-x
```

The memory directory lives at:
```
~/.claude/projects/<encoded-path>/memory/
```

## Behavior

### On `wt new` (worktree creation)

1. Resolve main repo's memory dir: `~/.claude/projects/<encode(repo-root)>/memory/`
2. Resolve new worktree's memory dir: `~/.claude/projects/<encode(worktree-path)>/memory/`
3. If main's memory dir exists and has content:
   - Copy entire directory to worktree's memory location (seeding)
   - Save snapshot of the memory directory for three-way merge later
4. If main has no memory dir: do nothing (worktree starts fresh)

### On `wt remove` / `wt prune` (worktree removal)

1. If worktree has no memory dir or it's empty: nothing to do
2. If worktree has memory content:
   - **Main also has memory**: detect changes, offer merge-back prompt (same UI as
     config files). Individual files get three-way merge when mergiraf + snapshot
     are available; subdirectories get plain copy.
   - **Main has no memory**: straight copy the entire memory directory to main's
     memory location (no merge needed, no conflict possible).
3. Clean up memory snapshot after removal.

### Edge Case Matrix

| Main has memory? | Worktree has memory? | On create       | On remove            |
|------------------|----------------------|-----------------|----------------------|
| Yes              | (seeded from main)   | Copy + snapshot  | Three-way merge      |
| No               | Yes (Claude created) | Nothing          | Copy to main         |
| No               | No                   | Nothing          | Nothing              |
| Yes              | Unchanged from seed  | Copy + snapshot  | No changes detected  |

## Implementation

### New function: `ClaudeProjectDir`

Encodes a filesystem path to the Claude Code project directory name. Returns the full
path to `~/.claude/projects/<encoded>/memory/`.

### Changes to `Manager`

Add methods:
- `CopyMemory(wtPath string) error` — copies main's memory dir to worktree's Claude
  project memory location. Called from `wt new` after worktree creation.
- `SaveMemorySnapshot(branch string) error` — snapshots main's memory dir for
  three-way merge. Called alongside `SaveSnapshot`.
- `DetectMemoryChanges(wtPath, branch string) ([]FileChange, error)` — compares
  worktree's memory dir against snapshot/main. Called from remove/prune.
- `MergeMemoryBack(wtPath, branch string) []MergeResult` — merges memory files back
  to main's memory dir. Uses existing `MergeBack` logic per-file.
- `RemoveMemorySnapshot(branch string) error` — cleans up memory snapshots. Called
  alongside `RemoveSnapshot`.

### Changes to CLI commands

**`cmd/wt/new.go`:**
- After `CopyFiles` + `SaveSnapshot`, call `CopyMemory` + `SaveMemorySnapshot`
- Non-blocking: warn on failure but continue

**`cmd/wt/remove.go` and `cmd/wt/prune.go`:**
- After config file change detection, also run `DetectMemoryChanges`
- Include memory changes in the merge-back prompt
- Handle the "main has no memory" case with a straight copy
- Call `RemoveMemorySnapshot` after removal

**`cmd/wt/config_changes.go`:**
- Memory changes appear in the same change list as config files
- Same merge/keep/skip/abort options apply

### Snapshot storage

Memory snapshots use the same location as config file snapshots:
```
~/.local/share/wt/snapshots/<repo>/<branch>/claude-memory/
```

The `claude-memory/` subdirectory keeps them separate from config file snapshots
(which mirror the repo's file structure).

## What Doesn't Change

- The `copy_files` config — memory handling is automatic, not user-configured
- The merge-back UI — same prompt, memory files appear alongside config files
- The three-way merge engine — same mergiraf integration, same fallback to copy
- Existing config file handling — this is additive

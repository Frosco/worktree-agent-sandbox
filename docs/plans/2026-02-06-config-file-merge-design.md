# Config File Three-Way Merge Design

## Problem

When removing or pruning worktrees, `wt` detects modified config files (e.g., `.claude/`) and offers to merge them back. The current "merge back" is a plain file copy that overwrites the main worktree's version. If both sides changed, the file is skipped entirely, losing the worktree's changes.

This is especially painful for `.claude` files, which are gitignored (team policy) and frequently edited in multiple worktrees simultaneously.

## Solution

Add three-way merge support using file snapshots and mergiraf.

### Snapshot at Creation

When `wt new` copies config files to a new worktree, also save a snapshot of each file to a dedicated directory. This snapshot serves as the "base" version for three-way merge later.

**Storage location:** `~/.local/share/wt/snapshots/<repo>/<branch>/`

The snapshot preserves directory structure. For example, copying `.claude/settings.json` produces:
- Worktree: `~/.local/share/wt/worktrees/<repo>/<branch>/.claude/settings.json`
- Snapshot: `~/.local/share/wt/snapshots/<repo>/<branch>/.claude/settings.json`

### Merge on Remove/Prune

When `MergeBack` is called for a changed file, the merge strategy depends on what's available:

```
Has snapshot? ──no──> Current behavior (copy if clean, skip if conflict)
     │
    yes
     │
mergiraf available? ──no──> Current behavior
     │
    yes
     │
Run: mergiraf merge <base> <left> <right> -o <output>
     │
Exit 0 (clean merge) ──> Write merged result to main worktree
     │
Exit 1 (conflict) ──> Keep main worktree version, warn user
```

Where:
- `<base>` = snapshot file
- `<left>` = main worktree file (current version)
- `<right>` = removed worktree file (the version being merged back)

### File Type Handling

- **Text files with snapshot + mergiraf:** Three-way merge as described above
- **Files only in worktree (not in main):** Copy as today
- **Binary files:** Current behavior (copy if clean, skip if conflict)
- **No snapshot available:** Current behavior (graceful degradation for pre-existing worktrees)

### Cleanup

When a worktree is removed, delete its snapshot directory (`snapshots/<repo>/<branch>/`).

## Changes Required

### `internal/worktree/worktree.go`

1. **Add `SnapshotBase` field to `Manager`** (or derive from `WorktreeBase` by convention, e.g., sibling `snapshots/` directory).

2. **`SnapshotPath(branch string) string`** - returns snapshot directory path for a branch.

3. **`SaveSnapshot(wtPath string, files []string) error`** - copies files to snapshot directory. Called after `CopyFiles` in `wt new`.

4. **`RemoveSnapshot(branch string) error`** - deletes snapshot directory. Called after successful worktree removal.

5. **Update `MergeBack`** to accept a `MergeOptions` struct or add a new `MergeBackSmart` method that:
   - Checks for snapshot file
   - Checks for mergiraf binary
   - Runs three-way merge if both available
   - Falls back to copy if not
   - Returns merge result status (clean, conflict, copied, error)

### `cmd/wt/new.go`

- After `CopyFiles`, call `SaveSnapshot` with the same file list.

### `cmd/wt/config_changes.go`

- Update merge handler to use smart merge and display appropriate messages:
  - "Merged %s (3-way)" for clean merges
  - "Kept main version of %s (conflict)" for conflicts
  - "Copied %s" for fallback copies

### `cmd/wt/remove.go` and `cmd/wt/prune.go`

- After successful removal, call `RemoveSnapshot`.

## Testing

- Test snapshot creation alongside `CopyFiles`
- Test `MergeBack` with snapshot + mergiraf available (clean merge case)
- Test `MergeBack` with snapshot + mergiraf available (conflict case, keeps main)
- Test `MergeBack` fallback when no snapshot exists
- Test `MergeBack` fallback when mergiraf not available
- Test snapshot cleanup on remove
- Test snapshot directory structure matches worktree structure

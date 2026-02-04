# Force Remove Design

## Problem

`wt remove --force` currently only skips config file change detection prompts. It does not force git to remove worktrees with uncommitted changes or untracked files. Users expect `--force` to mean "just delete it."

## Solution

Rename and consolidate the flags:

| Flag | Config prompts | Dirty worktree |
|------|----------------|----------------|
| (none) | Shown | Blocked by git |
| `--skip-changes` | Skipped | Blocked by git |
| `-f, --force` | Skipped | Force removed |

`--force` implies `--skip-changes`. The `-f` short flag moves to `--force`.

## Implementation

### worktree.Manager.Remove()

Add force parameter that passes `--force` to git:

```go
func (m *Manager) Remove(branch string, force bool) error {
    args := []string{"worktree", "remove"}
    if force {
        args = append(args, "--force")
    }
    args = append(args, wtPath)
    cmd := exec.Command("git", args...)
    // ...
}
```

### remove.go

Two flags, where `--force` implies `--skip-changes`:

```go
var (
    removeForce       bool  // -f/--force
    removeSkipChanges bool  // --skip-changes
)

// Skip prompts if either flag is set
skipPrompts := removeForce || removeSkipChanges
```

### Help text

```
-f, --force         Force removal even if worktree has uncommitted changes
    --skip-changes  Skip config file change detection (without forcing git)
```

## Testing

- `TestManager_Remove_Force` - verify `--force` passed to git
- `TestManager_Remove_NoForce` - verify no `--force` when force=false
- `TestRemoveCmd_Force_DirtyWorktree` - uncommitted changes removed with `--force`
- `TestRemoveCmd_SkipChanges_DirtyWorktree` - `--skip-changes` alone fails on dirty
- `TestRemoveCmd_Force_SkipsPrompts` - `--force` doesn't prompt for config changes
- Update existing tests calling `Manager.Remove()` to pass new parameter

## Edge Cases

- `--force --skip-changes` together: Valid but redundant
- Scripts using `wt remove -f`: Now also forces git removal (expected behavior for automation)

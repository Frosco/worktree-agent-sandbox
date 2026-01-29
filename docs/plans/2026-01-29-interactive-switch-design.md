# Interactive Worktree Picker for `wt switch`

## Summary

When `wt switch` is called without a branch argument, display an interactive picker allowing the user to select from available worktrees using arrow-key navigation.

## Behavior

- **With argument** (`wt switch feature-auth`): Unchanged - switches to that branch/worktree
- **Without argument** (`wt switch`): Shows interactive picker, then switches to selected branch

The `--print-path` flag works with both modes.

## Selection List

The picker displays:
1. Main repo branch (first)
2. All worktrees managed by `wt` (alphabetical)

Display format: branch name only (clean and simple).

Example:
```
? Select worktree:
> main
  feature-auth
  fix-sandbox-mount
```

The picker appears even when only the main repo exists (one option).

## Implementation

### Changes to `cmd/wt/switch.go`

1. Change `cobra.ExactArgs(1)` to `cobra.MaximumNArgs(1)`

2. Add branch selection at start of `RunE`:
   ```go
   var branch string
   if len(args) == 0 {
       branch, err = runInteractivePicker(repoRoot, mgr)
       if err != nil {
           return err
       }
   } else {
       branch = args[0]
   }
   ```

3. New helper function:
   ```go
   func runInteractivePicker(repoRoot string, mgr *worktree.Manager) (string, error)
   ```
   - Gets main branch via `worktree.GetMainBranch(repoRoot)`
   - Gets worktrees via `mgr.List()`
   - Builds options with main branch first
   - Uses `huh.NewSelect()` for the picker
   - Returns selected branch name

### New Dependency

Add `github.com/charmbracelet/huh` to `go.mod`.

## Testing

1. **Unit test options-building logic** - Verify main branch appears first, all worktrees included
2. **Skip interactive TUI testing** - Requires real TTY; rely on manual testing
3. **Existing tests unchanged** - Tests passing branch argument still work

## Files Changed

- `cmd/wt/switch.go` - Add interactive picker logic
- `go.mod` / `go.sum` - Add huh dependency

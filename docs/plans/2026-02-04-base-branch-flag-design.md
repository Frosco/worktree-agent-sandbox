# Design: Base Branch Flag for `wt new`

## Summary

Add `-b/--base <branch>` flag to `wt new` that specifies what branch to base the new branch on. When not specified, current behavior (base on HEAD) is preserved.

**Usage:**
```bash
wt new feature-x -b develop    # Create feature-x based on develop
wt new feature-y --base main   # Same with long form
wt new feature-z               # No change - bases on HEAD
```

## Behavior

When `-b <base>` is specified:

1. **Check if new branch already exists** (local or remote) → Error: `branch 'feature-x' already exists, cannot apply --base`

2. **Resolve base branch:**
   - If base exists locally → use it
   - If not local → run `git fetch origin <base>` and check again
   - If still not found → Error: `base branch 'develop' not found locally or on origin`

3. **Create worktree** with: `git worktree add -b <new-branch> <path> <base>`

When `-b` is **not** specified, behavior is unchanged from current implementation.

## Implementation Changes

**`internal/worktree/worktree.go`:**
- Add `FetchBranch(branch string) error` method to fetch a specific branch from origin
- Modify `Create(branch string)` to `Create(branch, baseBranch string)` - empty baseBranch means current behavior

**`cmd/wt/new.go`:**
- Add `-b/--base` flag
- Add validation: if `-b` specified and branch already exists, error before calling `Create`
- Pass base branch to `mgr.Create()`

**Tests:**
- Test creating branch with base that exists locally
- Test creating branch with base that needs fetching
- Test error when base not found anywhere
- Test error when `-b` specified but new branch already exists

## Documentation Changes

**`README.md`** - Update the "Create a new worktree" section:

```bash
wt new feature-branch             # Create from current HEAD
wt new feature-branch -b develop  # Create based on develop branch
```

# wt switch: Create worktree from remote branch

## Context

After the v2 refactor, `wt` no longer creates worktrees — that was delegated to `claude --worktree`. However, one valuable workflow was lost: running `wt switch <name>` on a branch that exists on origin but not locally, which would create a worktree for it automatically. This was primarily used to check out teammates' MR branches for review.

## Scope

Add the ability for `wt switch <name>` (explicit name only, not the interactive picker) to create a worktree when the name matches a remote branch on origin.

## Behavior

Current flow:
1. Name provided or picked interactively
2. If worktree exists -> print path
3. If not -> error

New flow (explicit name argument only):
1. Name provided as argument
2. If worktree exists -> print path (unchanged)
3. If not -> fetch from origin, check if `origin/<name>` exists
4. If remote branch exists -> create worktree at `.claude/worktrees/<name>/`, checking out `origin/<name>` as local branch `<name>`, copy `.worktreeinclude` files, print path
5. If no remote branch either -> error: "no worktree or remote branch found"

The interactive picker flow is unchanged — it only shows existing local worktrees.

## Manager additions

### `Create(name, remoteBranch string) error`

Runs `git worktree add -b <name> .claude/worktrees/<name>/ <remoteBranch>` to create the worktree with a local branch tracking the remote.

### `CopyWorktreeInclude(name string) error`

Reads `.worktreeinclude` from repo root (one path per line, blank lines and `#` comments skipped). Copies each listed file/directory from the repo root to the corresponding path in the new worktree. If `.worktreeinclude` doesn't exist, no-op.

Existing `FetchPrune()` and `RemoteBranchExists()` methods cover fetch and remote detection — no changes needed.

## Error handling

- **Fetch fails** (network error, no remote) — return the error, don't create anything.
- **`git worktree add` fails** (branch collision, disk issue) — return the git error with context wrapping.
- **`.worktreeinclude` references missing file** — skip silently. These are gitignored files that may not exist on every machine.
- **`.worktreeinclude` copy fails partway** — return the error. The worktree is already created and usable, just missing convenience files.

No new sentinel errors. All errors use `fmt.Errorf` with `%w` wrapping, consistent with existing style.

## Testing

All tests use real git repos and filesystems, no mocks.

- **`Manager.Create`** — set up a repo with a remote branch, call `Create`, verify worktree directory exists in `.claude/worktrees/<name>/`, verify local branch exists and tracks the remote.
- **`Manager.CopyWorktreeInclude`** — test with `.worktreeinclude` listing files (verify copied), missing `.worktreeinclude` (no-op), entries that don't exist in repo root (skipped silently).
- **`switch` command** — end-to-end: name not found locally + remote branch exists -> worktree created + path printed. Name not found locally + no remote branch -> error message.

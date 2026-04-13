# wt - Git Worktree Navigator and Cleaner

A companion CLI for [Claude Code's worktree support](https://code.claude.com/docs/en/common-workflows.md#run-parallel-claude-code-sessions-with-git-worktrees). Provides interactive switching between worktrees and batch cleanup of stale ones.

## Why

Claude Code creates worktrees with `claude --worktree <name>` and handles config file copying (`.worktreeinclude`) and memory sharing natively. What it doesn't provide is a way to quickly navigate between worktrees from your shell or batch-clean old ones. That's what `wt` does.

## Installation

```bash
# Build from source
go build -o wt-bin ./cmd/wt

# Install to PATH
cp wt-bin ~/.local/bin/

# Add shell integration to your shell rc file
echo 'eval "$(wt-bin shell-init bash)"' >> ~/.bashrc
# or for zsh:
echo 'eval "$(wt-bin shell-init zsh)"' >> ~/.zshrc
```

## Usage

### Switch to a worktree

```bash
wt switch feature-auth
# cd into .claude/worktrees/feature-auth/

wt switch
# No argument: interactive picker to select from available worktrees
```

### List worktrees

```bash
wt list
# Shows all worktrees under .claude/worktrees/ with name, branch, and path
```

### Prune stale worktrees

```bash
wt prune
# Removes worktrees whose branches were deleted from remote
# Only considers branches with upstream tracking - local-only branches are never pruned
# Prompts before removing worktrees with uncommitted changes or unpushed commits
# Force-deletes both worktree and branch after confirmation

wt prune --dry-run
# Preview what would be pruned

wt prune --force
# Skip prompts for uncommitted changes
```

### Run in sandbox (Podman)

```bash
wt sandbox feature-auth
# Requires existing worktree (use 'claude --worktree feature-auth' to create one)
# Starts Podman container with worktree mounted
# Runs mise install && claude --dangerously-skip-permissions

wt sandbox --no-claude    # Just get a shell
wt sandbox --no-mise      # Skip mise install
wt sandbox -m ~/other-repo  # Mount additional paths
```

## Per-repo setup

For repos where `CLAUDE.md` or other gitignored files should be copied to worktrees, add a `.worktreeinclude` file to the repo root:

```
CLAUDE.md
.envrc
mise.local.toml
```

This is a Claude Code feature — files matching these patterns that are also gitignored get copied when `claude --worktree` creates a worktree.

## Development

### Git hooks

Pre-commit hooks live in `.githooks/` and check `goimports` formatting and `go vet`. To enable them after cloning:

```bash
git config core.hooksPath .githooks
```

Requires [`goimports`](https://pkg.go.dev/golang.org/x/tools/cmd/goimports):

```bash
go install golang.org/x/tools/cmd/goimports@latest
```

## Requirements

- Go 1.24+
- Git
- Podman (for sandbox feature)

## License

MIT

# wt - Git Worktree Manager with Claude Code Sandbox

A CLI tool to manage git worktrees with automatic config file copying and Podman container support for running Claude Code in isolation.

## Features

- Create and switch between git worktrees with a single command
- Automatically copy gitignored config files (like `CLAUDE.md`, `mise.local.toml`) to new worktrees
- Detect changes in config files when removing worktrees and offer to merge them back
- Run Claude Code in a sandboxed Podman container with `--dangerously-skip-permissions`
- Shell integration for seamless `cd` into worktrees

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

### Create a new worktree

```bash
wt new feature-branch
# Creates worktree at ~/.local/share/wt/worktrees/<repo>/feature-branch
# Copies configured files from main worktree
# cd's into the new worktree (via shell function)
```

### Switch to a worktree

```bash
wt switch feature-branch
# If worktree exists: cd into it
# If branch exists but no worktree: create worktree, then cd into it
# If branch doesn't exist: error (use 'wt new' to create new branches)
```

### List worktrees

```bash
wt list
# Shows all worktrees for the current repo
```

### Remove a worktree

```bash
wt remove feature-branch
# Detects if config files were modified
# Offers to merge changes back to main worktree
# Use --force to skip change detection
```

### Run in sandbox

```bash
wt sandbox feature-branch
# Creates/switches to worktree
# Starts Podman container with worktree mounted
# Runs mise install && claude --dangerously-skip-permissions

# Options:
wt sandbox --no-claude    # Just get a shell
wt sandbox --no-mise      # Skip mise install
wt sandbox -m ~/other-repo  # Mount additional paths
```

## Configuration

### Global config: `~/.config/wt/config.toml`

```toml
# Files to copy to new worktrees
copy_files = ["CLAUDE.md", ".envrc", "mise.local.toml"]

# Additional paths to mount in sandbox
extra_mounts = ["~/shared-libs", "~/data:ro"]
```

### Per-repo config: `.wt.toml` in repo root

```toml
# Repo-specific files to copy (merged with global)
copy_files = [".env.local"]

# Repo-specific mounts
extra_mounts = ["~/work/common-deps"]
```

## Directory Structure

Worktrees are stored in XDG-compliant locations:

```
~/.local/share/wt/worktrees/
└── <repo-name>/
    ├── feature-a/
    ├── feature-b/
    └── bugfix-123/
```

## Requirements

- Go 1.21+
- Git
- Podman (for sandbox feature)

## License

MIT

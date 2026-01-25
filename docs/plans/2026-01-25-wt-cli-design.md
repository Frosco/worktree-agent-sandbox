# wt - Git Worktree Manager with Claude Code Sandbox

Design document for a CLI tool that streamlines git worktree management and provides isolated Claude Code execution environments.

## Problem

Working on multiple features simultaneously with Claude Code has friction:
1. Creating worktrees and copying tool-specific config files is manual
2. Running Claude with `--dangerously-skip-permissions` on the host is risky
3. Switching between worktrees requires remembering paths

## Solution

`wt` - a Go CLI that manages worktrees and provides Podman-based sandboxes for Claude Code.

## Commands

```
wt new <branch>        Create worktree, copy config files, cd into it
wt switch <branch>     Switch to worktree (create if needed), cd into it
wt list                List worktrees for current repo
wt remove <branch>     Remove worktree (detects config file changes)
wt sandbox [branch]    Run Claude in Podman container
wt-bin shell-init bash Output shell function for sourcing
```

### Shell Integration

Since subprocesses cannot change the parent shell's directory, `wt` uses a shell function wrapper:

```bash
# Add to ~/.bashrc
eval "$(wt-bin shell-init bash)"
```

This defines the `wt` function that wraps `wt-bin` and handles `cd` for `new` and `switch` commands.

## Directory Structure

### Worktree Location

```
~/.local/share/wt/worktrees/<repo-name>/<branch>/
```

Example:
```
~/.local/share/wt/worktrees/my-project/feature-auth/
```

### Config Files

**Global:** `~/.config/wt/config.toml`
```toml
# Files to copy to every new worktree
copy_files = ["CLAUDE.md", ".envrc"]

# Additional paths to mount in sandbox
extra_mounts = [
  "~/shared-libs",
  "~/reference-repo:ro"
]
```

**Per-repo:** `.wt.toml` in repo root (gitignored)
```toml
# Additional files for this repo
copy_files = ["mise.local.toml", ".env.local"]

# Extra mounts for this repo's sandboxes
extra_mounts = ["~/company-standards:ro"]
```

Per-repo config adds to global config (does not replace).

## Worktree Operations

### wt new

1. Determine repo from current directory
2. Create branch if it doesn't exist (from current HEAD)
3. Create worktree at `~/.local/share/wt/worktrees/<repo>/<branch>/`
4. Copy files listed in global + per-repo config from main worktree
5. cd into new worktree

Errors if worktree already exists.

### wt switch

1. If worktree exists, cd into it
2. If not, perform `wt new` logic

Idempotent - the everyday command.

### wt remove

1. Check if copied config files differ from source worktree
2. If changes detected:
   ```
   These files were modified in feature-x:
     CLAUDE.md (12 lines added)
     mise.local.toml (2 lines changed)

   [m] Merge back to main worktree
   [d] Show diff
   [k] Keep original (discard changes)
   [a] Abort remove
   ```
3. If source file also changed, warn and abort (no auto-merge on conflicts)
4. Run `git worktree remove`

### wt list

Show worktrees for current repo with branch names and paths.

## Sandbox (Podman Container)

### wt sandbox [branch]

1. If branch provided, run `wt switch` first
2. Build container image if not cached
3. Start container with mounts
4. Run `mise install` then `claude --dangerously-skip-permissions`

### Container Setup

**Mounts:**
```bash
podman run --userns=keep-id \
  -v "$WORKTREE_PATH:$WORKTREE_PATH:Z" \
  -v "$MAIN_GIT_DIR:$MAIN_GIT_DIR:ro" \
  -v "$HOME/.claude:/home/user/.claude:ro" \
  # extra_mounts from config...
  --dns=8.8.8.8 \
  -w "$WORKTREE_PATH" \
  -it wt-sandbox
```

Key points:
- `--userns=keep-id` maps host UID so created files are owned by user
- Worktree mounted at same absolute path (git worktree references work)
- Main repo `.git` mounted for worktree internals
- `~/.claude/` mounted read-only for OAuth credentials
- Extra mounts from global/per-repo config
- `--mount` flag for one-off additional mounts

**Container Image:**
- Base: Node.js (for Claude Code)
- Tools: mise, git, ripgrep, fd
- Built from Containerfile on first run, cached thereafter

### Extra Mounts

Config:
```toml
extra_mounts = ["~/other-repo", "~/data:ro"]
```

Flag (adds to config, doesn't replace):
```bash
wt sandbox --mount ~/another-repo --mount ~/secrets:ro
```

## Implementation

### Project Structure

```
wt/
├── cmd/
│   └── wt/
│       └── main.go
├── internal/
│   ├── config/      # TOML parsing, config merging
│   ├── worktree/    # Git worktree operations
│   ├── sandbox/     # Podman container management
│   └── shell/       # Shell init script generation
├── Containerfile
├── go.mod
└── go.sum
```

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/pelletier/go-toml/v2` - TOML parsing
- Standard library `os/exec` for git/podman

### Installation

```bash
go build -o wt-bin ./cmd/wt
cp wt-bin ~/.local/bin/
echo 'eval "$(wt-bin shell-init bash)"' >> ~/.bashrc
```

## Error Handling

| Situation | Behavior |
|-----------|----------|
| Not in git repo | Error with message |
| Worktree exists (wt new) | Error, suggest `wt switch` |
| Branch doesn't exist | Create from current HEAD |
| Podman not installed | Error with install instructions |
| Container build fails | Show output, exit |
| Config file conflict on remove | Warn, abort, user handles manually |

## Out of Scope (MVP)

- VM support (Vagrant/libvirt) - add if Docker-in-Docker becomes an issue
- Docker runtime support - Podman only
- Auto-pruning stale worktrees
- Remote branch tracking
- 3-way merge for config conflicts

## Future Considerations

- `wt sandbox --no-claude` - shell into container without starting Claude
- `wt status` - show uncommitted changes across worktrees
- VM backend for projects that use Docker in development

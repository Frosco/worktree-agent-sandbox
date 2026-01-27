package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	opts := &Options{
		WorktreePath:   "/home/user/worktrees/myrepo/feature",
		MainGitDir:     "/home/user/dev/myrepo/.git",
		ClaudeDir:      "/home/user/.claude",
		ExtraMounts:    []string{"/home/user/shared", "/data:ro"},
		ContainerImage: "wt-sandbox",
	}

	args, err := opts.BuildArgs()
	if err != nil {
		t.Fatalf("BuildArgs failed: %v", err)
	}

	// Should contain userns keep-id
	found := false
	for _, arg := range args {
		if arg == "--userns=keep-id" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing --userns=keep-id")
	}

	// Check volume mounts
	argStr := strings.Join(args, " ")
	if !strings.Contains(argStr, "-v /home/user/worktrees/myrepo/feature:/home/user/worktrees/myrepo/feature:Z") {
		t.Error("missing worktree mount")
	}
	if !strings.Contains(argStr, "-v /home/user/.claude:/home/user/.claude:ro") {
		t.Error("missing claude dir mount")
	}

	// Check read-only extra mount
	if !strings.Contains(argStr, "-v /data:/data:ro") {
		t.Error("missing read-only extra mount")
	}

	// Check regular extra mount
	if !strings.Contains(argStr, "-v /home/user/shared:/home/user/shared:Z") {
		t.Error("missing regular extra mount")
	}
}

func TestBuildArgsTildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot get home dir: %v", err)
	}

	opts := &Options{
		WorktreePath:   "/tmp/test-worktree",
		ExtraMounts:    []string{"~/shared-libs", "~/data:ro"},
		ContainerImage: "wt-sandbox",
	}

	args, err := opts.BuildArgs()
	if err != nil {
		t.Fatalf("BuildArgs failed: %v", err)
	}

	argStr := strings.Join(args, " ")

	// Check tilde expanded for regular mount
	expectedPath := filepath.Join(home, "shared-libs")
	if !strings.Contains(argStr, expectedPath+":"+expectedPath+":Z") {
		t.Errorf("tilde not expanded for ~/shared-libs, got: %s", argStr)
	}

	// Check tilde expanded for read-only mount
	expectedROPath := filepath.Join(home, "data")
	if !strings.Contains(argStr, expectedROPath+":"+expectedROPath+":ro") {
		t.Errorf("tilde not expanded for ~/data:ro, got: %s", argStr)
	}
}

func TestBuildArgsMountsMiseDirs(t *testing.T) {
	opts := &Options{
		WorktreePath:   "/tmp/test-worktree",
		MiseDataDir:    "/home/user/.local/share/mise",
		MiseStateDir:   "/home/user/.local/state/mise",
		MiseCacheDir:   "/home/user/.cache/mise",
		ContainerImage: "wt-sandbox",
	}

	args, err := opts.BuildArgs()
	if err != nil {
		t.Fatalf("BuildArgs failed: %v", err)
	}

	argStr := strings.Join(args, " ")

	// Mise data dir should be mounted RW so installed tools persist
	if !strings.Contains(argStr, "-v /home/user/.local/share/mise:/home/user/.local/share/mise:Z") {
		t.Errorf("missing mise data dir mount, got: %s", argStr)
	}

	// Mise state dir should be mounted RW so it can persist trust decisions
	if !strings.Contains(argStr, "-v /home/user/.local/state/mise:/home/user/.local/state/mise:Z") {
		t.Errorf("missing mise state dir mount, got: %s", argStr)
	}

	// Mise cache dir should be mounted RW so downloaded tools persist
	if !strings.Contains(argStr, "-v /home/user/.cache/mise:/home/user/.cache/mise:Z") {
		t.Errorf("missing mise cache dir mount, got: %s", argStr)
	}
}

func TestBuildArgsSetsHomeEnvVar(t *testing.T) {
	opts := &Options{
		WorktreePath:   "/tmp/test-worktree",
		ClaudeDir:      "/home/user/.claude",
		ContainerImage: "wt-sandbox",
	}

	args, err := opts.BuildArgs()
	if err != nil {
		t.Fatalf("BuildArgs failed: %v", err)
	}

	argStr := strings.Join(args, " ")

	// HOME env var should be set to parent of ClaudeDir so Claude Code finds its config
	if !strings.Contains(argStr, "-e HOME=/home/user") {
		t.Errorf("missing HOME env var, got: %s", argStr)
	}
}

func TestPodmanAvailable(t *testing.T) {
	err := CheckPodmanAvailable()
	// This test depends on podman being installed
	if err != nil {
		t.Skipf("podman not available: %v", err)
	}
}

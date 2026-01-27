package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options configures the sandbox container
type Options struct {
	WorktreePath     string
	MainGitDir       string
	ClaudeDir        string
	ClaudeConfigFile string // ~/.claude.json global state file
	MiseDataDir      string
	MiseStateDir     string
	MiseCacheDir     string
	ExtraMounts      []string
	ContainerImage   string
	RunMiseInstall   bool
	StartClaude      bool
}

// CheckPodmanAvailable verifies podman is installed
func CheckPodmanAvailable() error {
	cmd := exec.Command("podman", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman not found. Install with: sudo pacman -S podman")
	}
	return nil
}

// BuildArgs constructs podman run arguments
func (o *Options) BuildArgs() ([]string, error) {
	args := []string{
		"run",
		"--rm",
		"-it",
		"--userns=keep-id",
		"--dns=8.8.8.8",
	}

	// Mount worktree at same path
	args = append(args, "-v", fmt.Sprintf("%s:%s:Z", o.WorktreePath, o.WorktreePath))

	// Mount main git dir read-only
	if o.MainGitDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s:ro", o.MainGitDir, o.MainGitDir))
	}

	// Mount claude dir read-write (Claude Code needs to write debug logs, history, etc.)
	if o.ClaudeDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s:Z", o.ClaudeDir, o.ClaudeDir))
		// Set HOME to parent of ClaudeDir so Claude Code finds its config
		homeDir := filepath.Dir(o.ClaudeDir)
		args = append(args, "-e", fmt.Sprintf("HOME=%s", homeDir))
	}

	// Mount claude global config file (~/.claude.json) read-write
	if o.ClaudeConfigFile != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s:Z", o.ClaudeConfigFile, o.ClaudeConfigFile))
	}

	// Mount mise directories read-write so tools and state persist
	if o.MiseDataDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s:Z", o.MiseDataDir, o.MiseDataDir))
	}
	if o.MiseStateDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s:Z", o.MiseStateDir, o.MiseStateDir))
	}
	if o.MiseCacheDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s:Z", o.MiseCacheDir, o.MiseCacheDir))
	}

	// Extra mounts
	for _, mount := range o.ExtraMounts {
		path := mount
		mode := "Z"
		if strings.HasSuffix(mount, ":ro") {
			path = strings.TrimSuffix(mount, ":ro")
			mode = "ro"
		}
		// Expand ~ to home directory
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("expanding ~ in mount %q: %w", mount, err)
			}
			path = filepath.Join(home, path[2:])
		}
		args = append(args, "-v", fmt.Sprintf("%s:%s:%s", path, path, mode))
	}

	// Working directory
	args = append(args, "-w", o.WorktreePath)

	// Image
	args = append(args, o.ContainerImage)

	return args, nil
}

// BuildImage builds the sandbox container image
func BuildImage(containerfilePath, imageName string) error {
	cmd := exec.Command("podman", "build", "-t", imageName, "-f", containerfilePath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ImageExists checks if an image exists locally
func ImageExists(imageName string) bool {
	cmd := exec.Command("podman", "image", "exists", imageName)
	return cmd.Run() == nil
}

// Run starts the sandbox container
func Run(opts *Options) error {
	args, err := opts.BuildArgs()
	if err != nil {
		return err
	}

	// Build the command to run inside
	var innerCmd string
	if opts.RunMiseInstall && opts.StartClaude {
		innerCmd = "mise install && claude --dangerously-skip-permissions"
	} else if opts.RunMiseInstall {
		innerCmd = "mise install && bash"
	} else if opts.StartClaude {
		innerCmd = "claude --dangerously-skip-permissions"
	} else {
		innerCmd = "bash"
	}

	args = append(args, "bash", "-c", innerCmd)

	cmd := exec.Command("podman", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

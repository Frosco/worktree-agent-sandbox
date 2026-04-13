package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/niref/wt/internal/sandbox"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	sandboxMounts   []string
	sandboxNoClaude bool
	sandboxNoMise   bool
	sandboxImage    string
)

var sandboxCmd = &cobra.Command{
	Use:   "sandbox [branch]",
	Short: "Run Claude Code in a sandboxed container",
	Long:  `Start a Podman container with the worktree mounted and run Claude with --dangerously-skip-permissions.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check podman is available
		if err := sandbox.CheckPodmanAvailable(); err != nil {
			return err
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		mgr := worktree.NewManager(repoRoot)

		var wtPath string

		if len(args) > 0 {
			name := args[0]
			if !mgr.Exists(name) {
				return fmt.Errorf("worktree %q does not exist (use 'claude --worktree %s' to create it)", name, name)
			}
			wtPath = mgr.WorktreePath(name)
		} else {
			// Use current directory
			wtPath = cwd
		}

		// Find the main .git directory
		mainGitDir := filepath.Join(repoRoot, ".git")

		// Claude credentials directory and config file
		home, _ := os.UserHomeDir()
		claudeDir := filepath.Join(home, ".claude")

		// Claude global config file (~/.claude.json) - only mount if exists
		claudeConfigFile := filepath.Join(home, ".claude.json")
		if _, err := os.Stat(claudeConfigFile); os.IsNotExist(err) {
			// Create empty file so Claude Code can write to it
			if f, err := os.Create(claudeConfigFile); err == nil {
				f.Close()
			}
		}

		// Mise directories (for persisting installed tools, trust state, and cache)
		miseDataDir := filepath.Join(home, ".local", "share", "mise")
		miseStateDir := filepath.Join(home, ".local", "state", "mise")
		miseCacheDir := filepath.Join(home, ".cache", "mise")

		allMounts := sandboxMounts

		// Build/check image
		imageName := sandboxImage
		if imageName == "" {
			imageName = "wt-sandbox"
		}

		if !sandbox.ImageExists(imageName) {
			fmt.Fprintln(cmd.OutOrStdout(), "Building sandbox image (this may take a few minutes)...")
			containerfile := findContainerfile(repoRoot)
			if containerfile == "" {
				return fmt.Errorf("Containerfile not found. Place it at <repo>/Containerfile or ~/.local/share/wt/Containerfile, or specify --image")
			}
			if err := sandbox.BuildImage(containerfile, imageName); err != nil {
				return fmt.Errorf("building image: %w", err)
			}
		}

		opts := &sandbox.Options{
			WorktreePath:     wtPath,
			MainGitDir:       mainGitDir,
			ClaudeDir:        claudeDir,
			ClaudeConfigFile: claudeConfigFile,
			MiseDataDir:      miseDataDir,
			MiseStateDir:     miseStateDir,
			MiseCacheDir:     miseCacheDir,
			ExtraMounts:      allMounts,
			ContainerImage:   imageName,
			RunMiseInstall:   !sandboxNoMise,
			StartClaude:      !sandboxNoClaude,
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Starting sandbox in %s...\n", wtPath)
		return sandbox.Run(opts)
	},
}

func init() {
	sandboxCmd.Flags().StringArrayVarP(&sandboxMounts, "mount", "m", nil, "Additional paths to mount")
	sandboxCmd.Flags().BoolVar(&sandboxNoClaude, "no-claude", false, "Don't start Claude, just get a shell")
	sandboxCmd.Flags().BoolVar(&sandboxNoMise, "no-mise", false, "Don't run mise install")
	sandboxCmd.Flags().StringVar(&sandboxImage, "image", "", "Container image to use")
	rootCmd.AddCommand(sandboxCmd)
}

// findContainerfile looks for a Containerfile in the repo root or ~/.local/share/wt/.
func findContainerfile(repoRoot string) string {
	candidates := []string{
		filepath.Join(repoRoot, "Containerfile"),
	}
	home, err := os.UserHomeDir()
	if err == nil {
		candidates = append(candidates, filepath.Join(home, ".local", "share", "wt", "Containerfile"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

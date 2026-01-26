package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/sandbox"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	sandboxMounts       []string
	sandboxWorktreeBase string
	sandboxConfigPath   string
	sandboxNoClaude     bool
	sandboxNoMise       bool
	sandboxImage        string
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

		paths := config.DefaultPaths()
		worktreeBase := sandboxWorktreeBase
		if worktreeBase == "" {
			worktreeBase = paths.WorktreeBase
		}
		configPath := sandboxConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		// Load config for extra mounts (errors intentionally ignored - sandbox should work even without config)
		globalCfg, _ := config.LoadGlobalConfig(configPath)
		repoCfg, _ := config.LoadRepoConfig(repoRoot)
		cfg := config.MergeConfigs(globalCfg, repoCfg)

		mgr := worktree.NewManager(repoRoot, worktreeBase)

		var wtPath string

		if len(args) > 0 {
			branch := args[0]
			// Switch to (or create) worktree
			if mgr.Exists(branch) {
				wtPath = mgr.WorktreePath(branch)
			} else {
				var err error
				wtPath, err = mgr.Create(branch)
				if err != nil {
					return err
				}
				// Copy config files
				if len(cfg.CopyFiles) > 0 {
					mgr.CopyFiles(wtPath, cfg.CopyFiles)
				}
			}
		} else {
			// Use current directory if it's a worktree managed by us
			wtPath = cwd
		}

		// Find the main .git directory
		mainGitDir := filepath.Join(repoRoot, ".git")

		// Claude credentials directory
		home, _ := os.UserHomeDir()
		claudeDir := filepath.Join(home, ".claude")

		// Combine extra mounts from config and flags
		allMounts := append(cfg.ExtraMounts, sandboxMounts...)

		// Build/check image
		imageName := sandboxImage
		if imageName == "" {
			imageName = "wt-sandbox"
		}

		if !sandbox.ImageExists(imageName) {
			fmt.Fprintln(cmd.OutOrStdout(), "Building sandbox image (this may take a few minutes)...")
			// Look for Containerfile in repo root or installed location
			containerfile := filepath.Join(repoRoot, "Containerfile")
			if _, err := os.Stat(containerfile); os.IsNotExist(err) {
				return fmt.Errorf("Containerfile not found. Run from wt repo or specify --image")
			}
			if err := sandbox.BuildImage(containerfile, imageName); err != nil {
				return fmt.Errorf("building image: %w", err)
			}
		}

		opts := &sandbox.Options{
			WorktreePath:   wtPath,
			MainGitDir:     mainGitDir,
			ClaudeDir:      claudeDir,
			ExtraMounts:    allMounts,
			ContainerImage: imageName,
			RunMiseInstall: !sandboxNoMise,
			StartClaude:    !sandboxNoClaude,
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Starting sandbox in %s...\n", wtPath)
		return sandbox.Run(opts)
	},
}

func init() {
	sandboxCmd.Flags().StringArrayVarP(&sandboxMounts, "mount", "m", nil, "Additional paths to mount")
	sandboxCmd.Flags().StringVar(&sandboxWorktreeBase, "worktree-base", "", "Override worktree base directory")
	sandboxCmd.Flags().StringVar(&sandboxConfigPath, "config", "", "Override global config path")
	sandboxCmd.Flags().BoolVar(&sandboxNoClaude, "no-claude", false, "Don't start Claude, just get a shell")
	sandboxCmd.Flags().BoolVar(&sandboxNoMise, "no-mise", false, "Don't run mise install")
	sandboxCmd.Flags().StringVar(&sandboxImage, "image", "", "Container image to use")
	rootCmd.AddCommand(sandboxCmd)
}

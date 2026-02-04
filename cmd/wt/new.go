package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	newPrintPath    bool
	newWorktreeBase string
	newConfigPath   string
	newBaseBranch   string
)

var newCmd = &cobra.Command{
	Use:   "new <branch>",
	Short: "Create a new worktree for a branch",
	Long:  `Creates a worktree and copies config files. Creates branch if it doesn't exist.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]

		// Find repo root from current directory
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		// Determine paths
		paths := config.DefaultPaths()
		worktreeBase := newWorktreeBase
		if worktreeBase == "" {
			worktreeBase = paths.WorktreeBase
		}
		configPath := newConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		// Load configs
		globalCfg, err := config.LoadGlobalConfig(configPath)
		if err != nil {
			return fmt.Errorf("loading global config: %w", err)
		}
		repoCfg, err := config.LoadRepoConfig(repoRoot)
		if err != nil {
			return fmt.Errorf("loading repo config: %w", err)
		}
		cfg := config.MergeConfigs(globalCfg, repoCfg)

		// Create worktree
		mgr := worktree.NewManager(repoRoot, worktreeBase)
		wtPath, err := mgr.Create(branch, newBaseBranch)
		if err != nil {
			if errors.Is(err, worktree.ErrWorktreeExists) {
				return fmt.Errorf("worktree already exists, use 'wt switch %s' instead", branch)
			}
			return err
		}

		// Copy config files
		if len(cfg.CopyFiles) > 0 {
			copied, err := mgr.CopyFiles(wtPath, cfg.CopyFiles)
			if err != nil {
				return fmt.Errorf("copying files: %w", err)
			}
			if !newPrintPath && len(copied) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Copied: %v\n", copied)
			}
		}

		if newPrintPath {
			fmt.Fprintln(cmd.OutOrStdout(), wtPath)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Created worktree at %s\n", wtPath)
		}

		return nil
	},
}

func init() {
	newCmd.Flags().BoolVar(&newPrintPath, "print-path", false, "Only print the worktree path (for shell integration)")
	newCmd.Flags().StringVar(&newWorktreeBase, "worktree-base", "", "Override worktree base directory")
	newCmd.Flags().StringVar(&newConfigPath, "config", "", "Override global config path")
	newCmd.Flags().StringVarP(&newBaseBranch, "base", "b", "", "Base branch for the new branch")
	rootCmd.AddCommand(newCmd)
}

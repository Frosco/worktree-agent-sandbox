package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	switchPrintPath    bool
	switchWorktreeBase string
	switchConfigPath   string
)

var switchCmd = &cobra.Command{
	Use:   "switch <branch>",
	Short: "Switch to a worktree (create if needed)",
	Long:  `Switch to an existing worktree or create one if it doesn't exist.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		paths := config.DefaultPaths()
		worktreeBase := switchWorktreeBase
		if worktreeBase == "" {
			worktreeBase = paths.WorktreeBase
		}
		configPath := switchConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		mgr := worktree.NewManager(repoRoot, worktreeBase)

		// If worktree exists, just return its path
		if mgr.Exists(branch) {
			wtPath := mgr.WorktreePath(branch)
			if switchPrintPath {
				fmt.Fprintln(cmd.OutOrStdout(), wtPath)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Switched to %s\n", wtPath)
			}
			return nil
		}

		// Otherwise, create it (same logic as new)
		globalCfg, err := config.LoadGlobalConfig(configPath)
		if err != nil {
			return fmt.Errorf("loading global config: %w", err)
		}
		repoCfg, err := config.LoadRepoConfig(repoRoot)
		if err != nil {
			return fmt.Errorf("loading repo config: %w", err)
		}
		cfg := config.MergeConfigs(globalCfg, repoCfg)

		wtPath, err := mgr.Create(branch)
		if err != nil {
			return err
		}

		if len(cfg.CopyFiles) > 0 {
			copied, err := mgr.CopyFiles(wtPath, cfg.CopyFiles)
			if err != nil {
				return fmt.Errorf("copying files: %w", err)
			}
			if !switchPrintPath && len(copied) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Copied: %v\n", copied)
			}
		}

		if switchPrintPath {
			fmt.Fprintln(cmd.OutOrStdout(), wtPath)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Created and switched to %s\n", wtPath)
		}

		return nil
	},
}

func init() {
	switchCmd.Flags().BoolVar(&switchPrintPath, "print-path", false, "Only print the worktree path")
	switchCmd.Flags().StringVar(&switchWorktreeBase, "worktree-base", "", "Override worktree base directory")
	switchCmd.Flags().StringVar(&switchConfigPath, "config", "", "Override global config path")
	rootCmd.AddCommand(switchCmd)
}

package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	removeWorktreeBase string
	removeConfigPath   string
	removeForce        bool
	removeSkipChanges  bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <branch>",
	Short: "Remove a worktree",
	Long:  `Remove a worktree. Detects config file changes and prompts for action.`,
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
		worktreeBase := removeWorktreeBase
		if worktreeBase == "" {
			worktreeBase = paths.WorktreeBase
		}
		configPath := removeConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		mgr := worktree.NewManager(repoRoot, worktreeBase)

		if !mgr.Exists(branch) {
			return fmt.Errorf("worktree '%s' does not exist", branch)
		}

		wtPath := mgr.WorktreePath(branch)

		// Check for config file changes (unless --force or --skip-changes)
		if !removeForce && !removeSkipChanges {
			// Config errors are intentionally ignored here - we still want to allow
			// removing a worktree even if config files are missing or malformed
			globalCfg, _ := config.LoadGlobalConfig(configPath)
			repoCfg, _ := config.LoadRepoConfig(repoRoot)
			cfg := config.MergeConfigs(globalCfg, repoCfg)

			if len(cfg.CopyFiles) > 0 {
				changes, err := mgr.DetectChanges(wtPath, cfg.CopyFiles)
				if err != nil {
					return fmt.Errorf("detecting changes: %w", err)
				}

				action := HandleConfigChanges(changes, mgr, wtPath, branch, cmd.OutOrStdout(), cmd.ErrOrStderr(), ConfigChangeOptions{
					AllowSkip:  false,
					AbortLabel: "Abort remove",
				})
				switch action {
				case ConfigChangeAbort:
					return fmt.Errorf("aborted")
				case ConfigChangeError:
					return fmt.Errorf("invalid choice")
				}
			}
		}

		if err := mgr.Remove(branch, removeForce); err != nil {
			return err
		}

		// Clean up snapshots
		if err := mgr.RemoveSnapshot(branch); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to remove snapshots: %v\n", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree '%s'\n", branch)
		return nil
	},
}

func init() {
	removeCmd.Flags().StringVar(&removeWorktreeBase, "worktree-base", "", "Override worktree base directory")
	removeCmd.Flags().StringVar(&removeConfigPath, "config", "", "Override global config path")
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Force removal even if worktree has uncommitted changes")
	removeCmd.Flags().BoolVar(&removeSkipChanges, "skip-changes", false, "Skip config file change detection")
	rootCmd.AddCommand(removeCmd)
}

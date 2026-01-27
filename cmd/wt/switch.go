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
	Short: "Switch to a worktree for an existing branch",
	Long:  `Switch to a worktree for an existing branch. Creates the worktree if needed. The branch must exist locally or on origin. Use 'wt new' to create a new branch.`,
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

		// If switching to the branch currently checked out in main repo, return main repo path
		mainBranch, err := worktree.GetMainBranch(repoRoot)
		if err == nil && branch == mainBranch {
			if switchPrintPath {
				fmt.Fprintln(cmd.OutOrStdout(), repoRoot)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Switched to %s\n", repoRoot)
			}
			return nil
		}

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

		// Check if branch exists locally or on remote - if neither, fail (don't auto-create)
		if !mgr.BranchExists(branch) && !mgr.RemoteBranchExists(branch) {
			return fmt.Errorf("branch %q does not exist locally or on origin (use 'wt new' to create a new branch)", branch)
		}

		// Branch exists but no worktree - create worktree for it
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

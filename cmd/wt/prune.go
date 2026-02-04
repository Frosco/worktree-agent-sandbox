package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/config"
	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	pruneWorktreeBase string
	pruneConfigPath   string
	pruneForce        bool
	pruneSkipChanges  bool
	pruneNoFetch      bool
	pruneDryRun       bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove worktrees for branches deleted from remote",
	Long: `Remove worktrees whose branches have been deleted from the remote (merged or manually deleted).

Only considers branches with upstream tracking configured - local-only branches are never pruned.
Prompts for worktrees with uncommitted changes or config file modifications.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		paths := config.DefaultPaths()
		wtBase := pruneWorktreeBase
		if wtBase == "" {
			wtBase = paths.WorktreeBase
		}

		mgr := worktree.NewManager(repoRoot, wtBase)

		// Fetch and prune remote refs (unless --no-fetch)
		if !pruneNoFetch {
			if err := mgr.FetchPrune(); err != nil {
				return fmt.Errorf("fetch failed: %w\nUse --no-fetch to skip fetching", err)
			}
		}

		// Get all worktrees
		worktrees, err := mgr.List()
		if err != nil {
			return err
		}

		if len(worktrees) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found")
			return nil
		}

		// Find prune candidates
		var candidates []worktree.WorktreeInfo
		for _, wt := range worktrees {
			upstream := mgr.BranchUpstream(wt.Branch)
			if upstream == "" {
				// No upstream tracking - skip (local-only branch)
				continue
			}
			// Check if upstream remote ref still exists
			if mgr.RemoteBranchExists(wt.Branch) {
				// Remote branch still exists - not a prune candidate
				continue
			}
			candidates = append(candidates, wt)
		}

		if len(candidates) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune")
			return nil
		}

		// Dry-run mode
		if pruneDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "Would prune (dry-run):")
			for _, c := range candidates {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", c.Branch)
			}
			return nil
		}

		// TODO: Implement actual pruning with prompts (Task 7)
		fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune")
		return nil
	},
}

func init() {
	pruneCmd.Flags().StringVar(&pruneWorktreeBase, "worktree-base", "", "Override worktree base directory")
	pruneCmd.Flags().StringVar(&pruneConfigPath, "config", "", "Override global config path")
	pruneCmd.Flags().BoolVarP(&pruneForce, "force", "f", false, "Force removal even if worktrees have uncommitted changes")
	pruneCmd.Flags().BoolVar(&pruneSkipChanges, "skip-changes", false, "Skip config file change detection")
	pruneCmd.Flags().BoolVar(&pruneNoFetch, "no-fetch", false, "Skip git fetch --prune (use current remote refs)")
	pruneCmd.Flags().BoolVarP(&pruneDryRun, "dry-run", "n", false, "Show what would be pruned without doing it")
	rootCmd.AddCommand(pruneCmd)
}

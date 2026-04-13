package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	pruneForce   bool
	pruneNoFetch bool
	pruneDryRun  bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove worktrees for branches deleted from remote",
	Long: `Remove worktrees whose branches have been deleted from the remote (merged or manually deleted).

Only considers branches with upstream tracking configured - local-only branches are never pruned.
Use --dry-run to preview what would be removed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoRoot, err := worktree.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not in a git repository")
		}

		mgr := worktree.NewManager(repoRoot)

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

		// Find prune candidates: worktrees whose branch has upstream tracking
		// but the remote branch no longer exists
		var candidates []worktree.WorktreeInfo
		for _, wt := range worktrees {
			if wt.Branch == "" {
				continue
			}
			upstream := mgr.BranchUpstream(wt.Branch)
			if upstream == "" {
				// No upstream tracking - skip (local-only branch)
				continue
			}
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
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s)\n", c.Name, c.Branch)
			}
			return nil
		}

		// Prune each candidate
		var pruned []string
		var errors []string

		for _, candidate := range candidates {
			name := candidate.Name
			wtPath := candidate.Path

			// Check for issues that require prompting
			hasUncommitted := mgr.HasUncommittedChanges(wtPath)
			hasUnpushed := mgr.HasUnpushedCommits(candidate.Branch)

			if (hasUncommitted || hasUnpushed) && !pruneForce {
				issues := []string{}
				if hasUncommitted {
					issues = append(issues, "uncommitted changes")
				}
				if hasUnpushed {
					issues = append(issues, "unpushed commits")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Remove %s? It has %s [y/n]: ", name, strings.Join(issues, " and "))

				reader := bufio.NewReader(os.Stdin)
				input, err := reader.ReadString('\n')
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s: failed to read input: %v", name, err))
					continue
				}
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "y" && input != "yes" {
					fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s\n", name)
					continue
				}
			}

			// Remove worktree (force: true because user confirmed or --force flag)
			if err := mgr.Remove(name, true); err != nil {
				errors = append(errors, fmt.Sprintf("%s: remove worktree: %v", name, err))
				continue
			}

			pruned = append(pruned, name)
		}

		// Print summary
		if len(pruned) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d worktree(s):\n", len(pruned))
			for _, p := range pruned {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
			}
		}

		if len(errors) > 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "\nErrors:")
			for _, e := range errors {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", e)
			}
		}

		if len(pruned) == 0 && len(errors) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to prune")
		}

		return nil
	},
}

func init() {
	pruneCmd.Flags().BoolVarP(&pruneForce, "force", "f", false, "Force removal even if worktrees have uncommitted changes")
	pruneCmd.Flags().BoolVar(&pruneNoFetch, "no-fetch", false, "Skip git fetch --prune (use current remote refs)")
	pruneCmd.Flags().BoolVarP(&pruneDryRun, "dry-run", "n", false, "Show what would be pruned without doing it")
	rootCmd.AddCommand(pruneCmd)
}

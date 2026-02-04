package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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

		// Prune each candidate
		var pruned []string
		var errors []string

		configPath := pruneConfigPath
		if configPath == "" {
			configPath = paths.GlobalConfig
		}

		for _, candidate := range candidates {
			branch := candidate.Branch
			wtPath := candidate.Path

			// Check for issues that require prompting
			hasUncommitted := mgr.HasUncommittedChanges(wtPath)
			hasUnpushed := mgr.HasUnpushedCommits(branch)

			if (hasUncommitted || hasUnpushed) && !pruneForce {
				issues := []string{}
				if hasUncommitted {
					issues = append(issues, "uncommitted changes")
				}
				if hasUnpushed {
					issues = append(issues, "unpushed commits")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Remove %s? It has %s [y/n]: ", branch, strings.Join(issues, " and "))

				reader := bufio.NewReader(os.Stdin)
				input, err := reader.ReadString('\n')
				if err != nil {
					errors = append(errors, fmt.Sprintf("%s: failed to read input: %v", branch, err))
					continue
				}
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "y" && input != "yes" {
					fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s\n", branch)
					continue
				}
			}

			// Config file change detection (unless --force or --skip-changes)
			if !pruneForce && !pruneSkipChanges {
				globalCfg, _ := config.LoadGlobalConfig(configPath)
				repoCfg, _ := config.LoadRepoConfig(repoRoot)
				cfg := config.MergeConfigs(globalCfg, repoCfg)

				if len(cfg.CopyFiles) > 0 {
					changes, err := mgr.DetectChanges(wtPath, cfg.CopyFiles)
					if err != nil {
						errors = append(errors, fmt.Sprintf("%s: detecting changes: %v", branch, err))
						continue
					}

					if len(changes) > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "\n%s has modified config files:\n", branch)
						for _, c := range changes {
							conflict := ""
							if c.Conflict {
								conflict = " (CONFLICT: source also changed)"
							}
							fmt.Fprintf(cmd.OutOrStdout(), "  %s%s\n", c.File, conflict)
						}
						fmt.Fprintln(cmd.OutOrStdout())
						fmt.Fprintln(cmd.OutOrStdout(), "[m] Merge back to main worktree")
						fmt.Fprintln(cmd.OutOrStdout(), "[k] Keep original (discard changes)")
						fmt.Fprintln(cmd.OutOrStdout(), "[s] Skip this worktree")
						fmt.Fprintln(cmd.OutOrStdout(), "[a] Abort prune")
						fmt.Fprint(cmd.OutOrStdout(), "Choice: ")

						reader := bufio.NewReader(os.Stdin)
						input, err := reader.ReadString('\n')
						if err != nil {
							errors = append(errors, fmt.Sprintf("%s: reading input: %v", branch, err))
							continue
						}
						input = strings.TrimSpace(strings.ToLower(input))

						switch input {
						case "m":
							for _, c := range changes {
								if c.Conflict {
									fmt.Fprintf(cmd.ErrOrStderr(), "Skipping %s due to conflict\n", c.File)
									continue
								}
								if err := mgr.MergeBack(wtPath, c.File); err != nil {
									fmt.Fprintf(cmd.ErrOrStderr(), "Failed to merge %s: %v\n", c.File, err)
								} else {
									fmt.Fprintf(cmd.OutOrStdout(), "Merged %s\n", c.File)
								}
							}
						case "k":
							// Continue with removal
						case "s":
							fmt.Fprintf(cmd.OutOrStdout(), "Skipping %s\n", branch)
							continue
						case "a":
							// Report what was already pruned before aborting
							if len(pruned) > 0 {
								fmt.Fprintf(cmd.OutOrStdout(), "\nPruned %d worktree(s) before abort:\n", len(pruned))
								for _, p := range pruned {
									fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
								}
							}
							return fmt.Errorf("aborted")
						default:
							errors = append(errors, fmt.Sprintf("%s: invalid choice", branch))
							continue
						}
					}
				}
			}

			// Remove worktree
			if err := mgr.Remove(branch, pruneForce); err != nil {
				errors = append(errors, fmt.Sprintf("%s: remove worktree: %v", branch, err))
				continue
			}

			// Delete local branch (force because remote is gone, so git sees it as "not fully merged")
			if err := mgr.DeleteBranch(branch, true); err != nil {
				// Worktree is already gone, just warn
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: removed worktree but failed to delete branch %s: %v\n", branch, err)
			}

			pruned = append(pruned, branch)
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
	pruneCmd.Flags().StringVar(&pruneWorktreeBase, "worktree-base", "", "Override worktree base directory")
	pruneCmd.Flags().StringVar(&pruneConfigPath, "config", "", "Override global config path")
	pruneCmd.Flags().BoolVarP(&pruneForce, "force", "f", false, "Force removal even if worktrees have uncommitted changes")
	pruneCmd.Flags().BoolVar(&pruneSkipChanges, "skip-changes", false, "Skip config file change detection")
	pruneCmd.Flags().BoolVar(&pruneNoFetch, "no-fetch", false, "Skip git fetch --prune (use current remote refs)")
	pruneCmd.Flags().BoolVarP(&pruneDryRun, "dry-run", "n", false, "Show what would be pruned without doing it")
	rootCmd.AddCommand(pruneCmd)
}

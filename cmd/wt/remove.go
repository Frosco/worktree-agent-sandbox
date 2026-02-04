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
	removeWorktreeBase string
	removeConfigPath   string
	removeForce        bool
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

		// Check for config file changes (unless --force)
		if !removeForce {
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

				if len(changes) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "These files were modified:")
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
					fmt.Fprintln(cmd.OutOrStdout(), "[a] Abort remove")
					fmt.Fprint(cmd.OutOrStdout(), "Choice: ")

					reader := bufio.NewReader(os.Stdin)
					input, err := reader.ReadString('\n')
					if err != nil {
						return fmt.Errorf("reading input: %w", err)
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
					case "a":
						return fmt.Errorf("aborted")
					default:
						return fmt.Errorf("invalid choice")
					}
				}
			}
		}

		if err := mgr.Remove(branch, removeForce); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree '%s'\n", branch)
		return nil
	},
}

func init() {
	removeCmd.Flags().StringVar(&removeWorktreeBase, "worktree-base", "", "Override worktree base directory")
	removeCmd.Flags().StringVar(&removeConfigPath, "config", "", "Override global config path")
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Skip change detection")
	rootCmd.AddCommand(removeCmd)
}

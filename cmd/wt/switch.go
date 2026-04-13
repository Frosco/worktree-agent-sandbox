package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var switchPrintPath bool

var switchCmd = &cobra.Command{
	Use:   "switch [name]",
	Short: "Switch to a worktree",
	Long: `Switch to a worktree by name. The name is the directory name under .claude/worktrees/.

If no name is specified, displays an interactive picker to select from available worktrees.`,
	Args: cobra.MaximumNArgs(1),
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

		// Determine name — either from argument or interactive picker
		var name string
		if len(args) == 0 {
			name, err = runInteractivePicker(repoRoot, mgr)
			if err != nil {
				return err
			}
		} else {
			name = args[0]
		}

		// "main" means the repo root itself
		mainBranch, err := worktree.GetMainBranch(repoRoot)
		if err == nil && name == mainBranch {
			if switchPrintPath {
				fmt.Fprintln(cmd.OutOrStdout(), repoRoot)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Switched to %s\n", repoRoot)
			}
			return nil
		}

		if !mgr.Exists(name) {
			return fmt.Errorf("worktree %q does not exist (use 'claude --worktree %s' to create it)", name, name)
		}

		wtPath := mgr.WorktreePath(name)
		if switchPrintPath {
			fmt.Fprintln(cmd.OutOrStdout(), wtPath)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Switched to %s\n", wtPath)
		}

		return nil
	},
}

func init() {
	switchCmd.Flags().BoolVar(&switchPrintPath, "print-path", false, "Only print the worktree path")
	rootCmd.AddCommand(switchCmd)
}

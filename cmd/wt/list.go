package main

import (
	"fmt"
	"os"

	"github.com/niref/wt/internal/worktree"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List worktrees for current repo",
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
		worktrees, err := mgr.List()
		if err != nil {
			return err
		}

		if len(worktrees) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No worktrees found")
			return nil
		}

		for _, wt := range worktrees {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", wt.Name, wt.Branch, wt.Path)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

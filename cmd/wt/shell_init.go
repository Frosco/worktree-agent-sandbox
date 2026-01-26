package main

import (
	"fmt"

	"github.com/niref/wt/internal/shell"
	"github.com/spf13/cobra"
)

var shellInitCmd = &cobra.Command{
	Use:   "shell-init [bash|zsh]",
	Short: "Output shell initialization script",
	Long:  `Output shell function for directory-changing commands. Add to your shell rc file.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		script := shell.GenerateInit(args[0])
		fmt.Fprint(cmd.OutOrStdout(), script)
	},
}

func init() {
	rootCmd.AddCommand(shellInitCmd)
}

package main

import (
	"github.com/charmbracelet/huh"
	"github.com/niref/wt/internal/worktree"
)

// buildPickerOptions creates the list of branch options for the interactive picker.
// Main branch is always first, followed by worktrees in the order provided.
func buildPickerOptions(mainBranch string, worktrees []worktree.WorktreeInfo) []string {
	options := make([]string, 0, 1+len(worktrees))
	options = append(options, mainBranch)
	for _, wt := range worktrees {
		options = append(options, wt.Branch)
	}
	return options
}

// runInteractivePicker displays an interactive picker and returns the selected branch.
func runInteractivePicker(repoRoot string, mgr *worktree.Manager) (string, error) {
	mainBranch, err := worktree.GetMainBranch(repoRoot)
	if err != nil {
		return "", err
	}

	worktrees, err := mgr.List()
	if err != nil {
		return "", err
	}

	options := buildPickerOptions(mainBranch, worktrees)

	huhOptions := make([]huh.Option[string], len(options))
	for i, opt := range options {
		huhOptions[i] = huh.NewOption(opt, opt)
	}

	var selected string
	err = huh.NewSelect[string]().
		Title("Select worktree").
		Options(huhOptions...).
		Value(&selected).
		Run()

	if err != nil {
		return "", err
	}

	return selected, nil
}

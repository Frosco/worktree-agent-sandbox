package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/niref/wt/internal/worktree"
)

// ConfigChangeAction represents the user's choice when config changes are detected
type ConfigChangeAction int

const (
	// ConfigChangeContinue means proceed with removal
	ConfigChangeContinue ConfigChangeAction = iota
	// ConfigChangeSkip means skip this worktree (only for batch operations)
	ConfigChangeSkip
	// ConfigChangeAbort means abort the entire operation
	ConfigChangeAbort
	// ConfigChangeError means an error occurred during prompting
	ConfigChangeError
)

// ConfigChangeOptions configures the behavior of HandleConfigChanges
type ConfigChangeOptions struct {
	// AllowSkip enables the [s] Skip option (for batch operations like prune)
	AllowSkip bool
	// BranchName is shown in the header (e.g., "feature-x has modified config files:")
	// If empty, shows generic "These files were modified:"
	BranchName string
	// AbortLabel customizes the abort option text (e.g., "Abort prune" vs "Abort remove")
	AbortLabel string
}

// HandleConfigChanges prompts the user about modified config files and handles their choice.
// Returns the action to take and performs merge-back if requested.
func HandleConfigChanges(
	changes []worktree.FileChange,
	mgr *worktree.Manager,
	wtPath string,
	branch string,
	stdout io.Writer,
	stderr io.Writer,
	opts ConfigChangeOptions,
) ConfigChangeAction {
	if len(changes) == 0 {
		return ConfigChangeContinue
	}

	// Display header
	if opts.BranchName != "" {
		fmt.Fprintf(stdout, "\n%s has modified config files:\n", opts.BranchName)
	} else {
		fmt.Fprintln(stdout, "These files were modified:")
	}

	// Display changes
	for _, c := range changes {
		conflict := ""
		if c.Conflict {
			conflict = " (CONFLICT: source also changed)"
		}
		fmt.Fprintf(stdout, "  %s%s\n", c.File, conflict)
	}
	fmt.Fprintln(stdout)

	// Display options
	fmt.Fprintln(stdout, "[m] Merge back to main worktree")
	fmt.Fprintln(stdout, "[k] Keep original (discard changes)")
	if opts.AllowSkip {
		fmt.Fprintln(stdout, "[s] Skip this worktree")
	}
	abortLabel := opts.AbortLabel
	if abortLabel == "" {
		abortLabel = "Abort"
	}
	fmt.Fprintf(stdout, "[a] %s\n", abortLabel)
	fmt.Fprint(stdout, "Choice: ")

	// Read input
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(stderr, "Error reading input: %v\n", err)
		return ConfigChangeError
	}
	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "m":
		for _, c := range changes {
			if c.Conflict {
				fmt.Fprintf(stderr, "Skipping %s due to conflict\n", c.File)
				continue
			}
			result := mgr.MergeBack(wtPath, c.File, branch)
			if result.Err != nil {
				fmt.Fprintf(stderr, "Failed to merge %s: %v\n", c.File, result.Err)
			} else {
				fmt.Fprintf(stdout, "Merged %s\n", c.File)
			}
		}
		return ConfigChangeContinue
	case "k":
		return ConfigChangeContinue
	case "s":
		if opts.AllowSkip {
			return ConfigChangeSkip
		}
		fmt.Fprintln(stderr, "Invalid choice")
		return ConfigChangeError
	case "a":
		return ConfigChangeAbort
	default:
		fmt.Fprintln(stderr, "Invalid choice")
		return ConfigChangeError
	}
}

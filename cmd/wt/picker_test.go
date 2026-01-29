package main

import (
	"testing"

	"github.com/niref/wt/internal/worktree"
)

func TestBuildPickerOptions(t *testing.T) {
	tests := []struct {
		name       string
		mainBranch string
		worktrees  []worktree.WorktreeInfo
		want       []string
	}{
		{
			name:       "main branch only",
			mainBranch: "main",
			worktrees:  nil,
			want:       []string{"main"},
		},
		{
			name:       "main branch with worktrees",
			mainBranch: "main",
			worktrees: []worktree.WorktreeInfo{
				{Branch: "feature-auth", Path: "/path/to/feature-auth"},
				{Branch: "fix-bug", Path: "/path/to/fix-bug"},
			},
			want: []string{"main", "feature-auth", "fix-bug"},
		},
		{
			name:       "master branch with worktrees",
			mainBranch: "master",
			worktrees: []worktree.WorktreeInfo{
				{Branch: "develop", Path: "/path/to/develop"},
			},
			want: []string{"master", "develop"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPickerOptions(tt.mainBranch, tt.worktrees)
			if len(got) != len(tt.want) {
				t.Errorf("buildPickerOptions() returned %d options, want %d", len(got), len(tt.want))
				return
			}
			for i, opt := range got {
				if opt != tt.want[i] {
					t.Errorf("buildPickerOptions()[%d] = %q, want %q", i, opt, tt.want[i])
				}
			}
		})
	}
}

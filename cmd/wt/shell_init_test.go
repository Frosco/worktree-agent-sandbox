package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestShellInitCommand(t *testing.T) {
	// Capture output
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"shell-init", "bash"})
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	}()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("shell-init failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "wt()") {
		t.Error("output should contain wt function")
	}
}

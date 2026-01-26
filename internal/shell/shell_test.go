package shell

import (
	"strings"
	"testing"
)

func TestGenerateBashInit(t *testing.T) {
	script := GenerateInit("bash")

	// Should define wt function
	if !strings.Contains(script, "wt()") {
		t.Error("script should define wt function")
	}

	// Should call wt-bin
	if !strings.Contains(script, "wt-bin") {
		t.Error("script should call wt-bin")
	}

	// Should handle new and switch with cd
	if !strings.Contains(script, "new|switch") {
		t.Error("script should handle new and switch commands")
	}

	// Should use cd
	if !strings.Contains(script, "cd ") {
		t.Error("script should use cd for directory changes")
	}
}

func TestGenerateZshInit(t *testing.T) {
	script := GenerateInit("zsh")

	// zsh version should also work
	if !strings.Contains(script, "wt()") {
		t.Error("script should define wt function")
	}
}

func TestGenerateUnknownShell(t *testing.T) {
	script := GenerateInit("fish")

	// Should return empty or error message for unsupported shells
	if script != "" && !strings.Contains(script, "not supported") {
		t.Error("unsupported shell should return empty or error")
	}
}

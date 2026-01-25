package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlobalConfig(t *testing.T) {
	// Create temp dir for test config
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "wt")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "config.toml")
	configContent := `
copy_files = ["CLAUDE.md", ".envrc"]
extra_mounts = ["~/shared-libs", "~/data:ro"]
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadGlobalConfig(configPath)
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	if len(cfg.CopyFiles) != 2 {
		t.Errorf("expected 2 copy_files, got %d", len(cfg.CopyFiles))
	}
	if cfg.CopyFiles[0] != "CLAUDE.md" {
		t.Errorf("expected CLAUDE.md, got %s", cfg.CopyFiles[0])
	}
	if len(cfg.ExtraMounts) != 2 {
		t.Errorf("expected 2 extra_mounts, got %d", len(cfg.ExtraMounts))
	}
}

func TestLoadGlobalConfigMissing(t *testing.T) {
	cfg, err := LoadGlobalConfig("/nonexistent/config.toml")
	if err != nil {
		t.Fatalf("missing config should not error: %v", err)
	}
	if cfg.CopyFiles != nil {
		t.Error("expected nil CopyFiles for missing config")
	}
}

func TestLoadGlobalConfigInvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.toml")
	// Invalid TOML syntax
	if err := os.WriteFile(configPath, []byte("not valid [ toml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadGlobalConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

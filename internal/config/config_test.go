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

func TestLoadRepoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".wt.toml")
	configContent := `
copy_files = ["mise.local.toml"]
extra_mounts = ["~/project-data"]
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadRepoConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadRepoConfig failed: %v", err)
	}

	if len(cfg.CopyFiles) != 1 || cfg.CopyFiles[0] != "mise.local.toml" {
		t.Errorf("unexpected copy_files: %v", cfg.CopyFiles)
	}
}

func TestMergeConfigs(t *testing.T) {
	global := &Config{
		CopyFiles:   []string{"CLAUDE.md", ".envrc"},
		ExtraMounts: []string{"~/shared"},
	}
	repo := &Config{
		CopyFiles:   []string{"mise.local.toml"},
		ExtraMounts: []string{"~/project"},
	}

	merged := MergeConfigs(global, repo)

	expectedFiles := []string{"CLAUDE.md", ".envrc", "mise.local.toml"}
	if len(merged.CopyFiles) != 3 {
		t.Errorf("expected 3 copy_files, got %d: %v", len(merged.CopyFiles), merged.CopyFiles)
	}
	for i, f := range expectedFiles {
		if merged.CopyFiles[i] != f {
			t.Errorf("copy_files[%d]: expected %s, got %s", i, f, merged.CopyFiles[i])
		}
	}

	if len(merged.ExtraMounts) != 2 {
		t.Errorf("expected 2 extra_mounts, got %d", len(merged.ExtraMounts))
	}
}

func TestMergeConfigsWithNil(t *testing.T) {
	repo := &Config{CopyFiles: []string{"file.txt"}}

	merged := MergeConfigs(nil, repo)
	if len(merged.CopyFiles) != 1 {
		t.Errorf("expected 1 file, got %d", len(merged.CopyFiles))
	}

	merged = MergeConfigs(repo, nil)
	if len(merged.CopyFiles) != 1 {
		t.Errorf("expected 1 file, got %d", len(merged.CopyFiles))
	}
}

func TestDefaultPaths(t *testing.T) {
	// Override HOME for test
	origHome := os.Getenv("HOME")
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origDataHome := os.Getenv("XDG_DATA_HOME")

	os.Setenv("HOME", "/home/testuser")
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_DATA_HOME")
	defer func() {
		os.Setenv("HOME", origHome)
		if origConfigHome != "" {
			os.Setenv("XDG_CONFIG_HOME", origConfigHome)
		}
		if origDataHome != "" {
			os.Setenv("XDG_DATA_HOME", origDataHome)
		}
	}()

	paths := DefaultPaths()

	expectedConfig := "/home/testuser/.config/wt/config.toml"
	if paths.GlobalConfig != expectedConfig {
		t.Errorf("GlobalConfig: expected %s, got %s", expectedConfig, paths.GlobalConfig)
	}

	expectedWorktrees := "/home/testuser/.local/share/wt/worktrees"
	if paths.WorktreeBase != expectedWorktrees {
		t.Errorf("WorktreeBase: expected %s, got %s", expectedWorktrees, paths.WorktreeBase)
	}
}

func TestDefaultPathsWithXDG(t *testing.T) {
	origHome := os.Getenv("HOME")
	origConfigHome := os.Getenv("XDG_CONFIG_HOME")
	origDataHome := os.Getenv("XDG_DATA_HOME")

	os.Setenv("HOME", "/home/testuser")
	os.Setenv("XDG_CONFIG_HOME", "/custom/config")
	os.Setenv("XDG_DATA_HOME", "/custom/data")
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("XDG_CONFIG_HOME", origConfigHome)
		os.Setenv("XDG_DATA_HOME", origDataHome)
	}()

	paths := DefaultPaths()

	if paths.GlobalConfig != "/custom/config/wt/config.toml" {
		t.Errorf("GlobalConfig with XDG: expected /custom/config/wt/config.toml, got %s", paths.GlobalConfig)
	}
	if paths.WorktreeBase != "/custom/data/wt/worktrees" {
		t.Errorf("WorktreeBase with XDG: expected /custom/data/wt/worktrees, got %s", paths.WorktreeBase)
	}
}

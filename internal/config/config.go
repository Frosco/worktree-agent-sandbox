package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// ErrContainerfileNotFound is returned when no Containerfile can be located
var ErrContainerfileNotFound = errors.New("Containerfile not found")

// Config represents wt configuration from TOML files
type Config struct {
	CopyFiles   []string `toml:"copy_files"`
	ExtraMounts []string `toml:"extra_mounts"`
}

// LoadGlobalConfig loads config from the given path.
// Returns empty config if file doesn't exist.
func LoadGlobalConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadRepoConfig loads .wt.toml from the given repo root.
// Returns empty config if file doesn't exist.
func LoadRepoConfig(repoRoot string) (*Config, error) {
	path := filepath.Join(repoRoot, ".wt.toml")
	return LoadGlobalConfig(path)
}

// MergeConfigs combines global and repo configs.
// Repo config adds to global (does not replace).
func MergeConfigs(global, repo *Config) *Config {
	merged := &Config{}

	if global != nil {
		merged.CopyFiles = append(merged.CopyFiles, global.CopyFiles...)
		merged.ExtraMounts = append(merged.ExtraMounts, global.ExtraMounts...)
	}
	if repo != nil {
		merged.CopyFiles = append(merged.CopyFiles, repo.CopyFiles...)
		merged.ExtraMounts = append(merged.ExtraMounts, repo.ExtraMounts...)
	}

	return merged
}

// Paths holds default file/directory paths
type Paths struct {
	GlobalConfig string
	WorktreeBase string
}

// DefaultPaths returns XDG-compliant default paths
func DefaultPaths() Paths {
	home := os.Getenv("HOME")

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}

	return Paths{
		GlobalConfig: filepath.Join(configHome, "wt", "config.toml"),
		WorktreeBase: filepath.Join(dataHome, "wt", "worktrees"),
	}
}

// FindContainerfile locates the Containerfile for building the sandbox image.
// It checks the XDG data directory first (~/.local/share/wt/Containerfile),
// then falls back to the repo root (for development).
func FindContainerfile(repoRoot string) (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home := os.Getenv("HOME")
		dataHome = filepath.Join(home, ".local", "share")
	}

	// Check XDG data dir first (installed location)
	dataContainerfile := filepath.Join(dataHome, "wt", "Containerfile")
	if _, err := os.Stat(dataContainerfile); err == nil {
		return dataContainerfile, nil
	}

	// Fall back to repo root (development location)
	repoContainerfile := filepath.Join(repoRoot, "Containerfile")
	if _, err := os.Stat(repoContainerfile); err == nil {
		return repoContainerfile, nil
	}

	return "", ErrContainerfileNotFound
}

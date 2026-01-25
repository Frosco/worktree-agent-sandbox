package config

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

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

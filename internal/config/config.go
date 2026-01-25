package config

import (
	"os"

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

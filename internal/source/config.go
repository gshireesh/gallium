package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config lists user-registered template sources, loaded from
// $XDG_CONFIG_HOME/gallium/config.yaml (default ~/.config/gallium/config.yaml).
type Config struct {
	Sources []SourceConfig `yaml:"sources"`
}

// SourceConfig is one named source: either a git repository (public or
// private; cloning uses the git CLI so the user's existing SSH/HTTPS
// credentials apply) or a local directory. Exactly one of Repo/Path is set.
type SourceConfig struct {
	Name string `yaml:"name"`
	Repo string `yaml:"repo,omitempty"`
	Path string `yaml:"path,omitempty"`
}

func ConfigPath() string {
	if p := os.Getenv("GALLIUM_CONFIG"); p != "" {
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "gallium", "config.yaml")
}

// LoadConfig returns an empty config when no config file exists.
func LoadConfig() (*Config, error) {
	path := ConfigPath()
	if path == "" {
		return &Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	for _, s := range cfg.Sources {
		if s.Name == "" || (s.Repo == "") == (s.Path == "") {
			return nil, fmt.Errorf("%s: each source needs a name and exactly one of repo or path", path)
		}
	}
	return &cfg, nil
}

func (c *Config) Find(name string) *SourceConfig {
	for i := range c.Sources {
		if c.Sources[i].Name == name {
			return &c.Sources[i]
		}
	}
	return nil
}

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
	}
	return path, nil
}

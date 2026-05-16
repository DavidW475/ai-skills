package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DirName             = ".ai-skills"
	SourcesFile         = "sources"
	InstalledFile       = "installed.yaml"
	ConfigFile          = "config.yaml"
	defaultSkillsSubdir = ".agent/skills"
)

// Config holds user-level configuration stored in ~/.ai-skills/config.yaml.
type Config struct {
	// SkillsDir is the directory where skills are installed.
	// Supports ~ expansion. Defaults to ~/.agent/skills when empty.
	SkillsDir string `yaml:"skills_dir,omitempty"`
}

// UserDir returns the absolute path to the user config directory (~/.ai-skills/).
func UserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: cannot determine home directory: %w", err)
	}
	return filepath.Join(home, DirName), nil
}

// Load reads ~/.ai-skills/config.yaml. Returns an empty Config if it does not exist.
func Load() (*Config, error) {
	dir, err := UserDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, ConfigFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return &c, nil
}

// Save writes the config to ~/.ai-skills/config.yaml, creating the directory if needed.
func Save(c *Config) error {
	dir, err := UserDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: mkdir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, ConfigFile), data, 0o644)
}

// SkillsInstallDir returns the resolved absolute path where skills are installed.
// Uses Config.SkillsDir if set (supports ~ expansion), otherwise ~/.agent/skills.
func (c *Config) SkillsInstallDir() (string, error) {
	if c.SkillsDir != "" {
		return expandHome(c.SkillsDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: cannot determine home directory: %w", err)
	}
	return filepath.Join(home, defaultSkillsSubdir), nil
}

func expandHome(path string) (string, error) {
	if len(path) > 1 && path[0] == '~' && (path[1] == '/' || path[1] == os.PathSeparator) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

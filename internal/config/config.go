package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Project ProjectConfig `toml:"project"`
	Claude  ClaudeConfig  `toml:"claude"`
}

type ProjectConfig struct {
	Path        string `toml:"path"`
	WorktreeBase string `toml:"worktree_base"`
}

type ClaudeConfig struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

func Load() (*Config, error) {
	cfg := defaults()

	path, err := configPath()
	if err != nil {
		return cfg, nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaults() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Project: ProjectConfig{
			Path:        filepath.Join(home, "project"),
			WorktreeBase: filepath.Join(home, "worktree"),
		},
		Claude: ClaudeConfig{
			Command: "claude",
			Args:    []string{"--dangerously-skip-permissions"},
		},
	}
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "crewalk", "config.toml"), nil
}

func (c *Config) WorktreePath(ticketID string) string {
	return filepath.Join(c.Project.WorktreeBase, "feature", ticketID)
}

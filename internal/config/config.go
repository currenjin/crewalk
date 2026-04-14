package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Project ProjectConfig `toml:"project"`
	Claude  ClaudeConfig  `toml:"claude"`
}

type ProjectConfig struct {
	Path         string `toml:"path"`
	WorktreeBase string `toml:"worktree_base"`
}

type ClaudeConfig struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

func Load() (*Config, error) {
	cfg, err := fromCwd()
	if err != nil {
		return nil, err
	}

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

// fromCwd detects the project root from the current working directory
// by walking up to find the nearest .git directory.
func fromCwd() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	projectPath, err := findGitRoot(cwd)
	if err != nil {
		return nil, fmt.Errorf("not inside a git repository (run crewalk from your project root): %w", err)
	}

	return &Config{
		Project: ProjectConfig{
			Path:         projectPath,
			WorktreeBase: filepath.Join(filepath.Dir(projectPath), "worktree"),
		},
		Claude: ClaudeConfig{
			Command: "claude",
			Args:    []string{"--dangerously-skip-permissions"},
		},
	}, nil
}

func findGitRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .git found")
		}
		dir = parent
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

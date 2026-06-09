package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Repos         []RepoConfig    `yaml:"repos"`
	PollInterval  Duration        `yaml:"poll_interval"`
	GitHub        GitHubConfig    `yaml:"github"`
	Notifications NotifyConfig    `yaml:"notifications"`
	Agents        []AgentConfig   `yaml:"agents"`
	DefaultAgent  string          `yaml:"default_agent"`
	Daemon        DaemonConfig    `yaml:"daemon"`
	Dashboard     DashboardConfig `yaml:"dashboard"`
}

type RepoConfig struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type GitHubConfig struct {
	TokenFrom string `yaml:"token_from"`
	Token     string `yaml:"token"`
	TokenEnv  string `yaml:"token_env"`
}

type NotifyConfig struct {
	Desktop DesktopNotifyConfig `yaml:"desktop"`
}

type DesktopNotifyConfig struct {
	Enabled bool     `yaml:"enabled"`
	Events  []string `yaml:"events"`
}

type AgentConfig struct {
	Name    string   `yaml:"name"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args,omitempty"`
}

type DaemonConfig struct {
	Socket string `yaml:"socket"`
}

type DashboardConfig struct {
	ItemsPerRepo int `yaml:"items_per_repo"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "ai-mux", "config.yaml")
	}
	return filepath.Join(home, ".config", "ai-mux", "config.yaml")
}

func Default() *Config {
	return &Config{
		PollInterval: Duration{30 * time.Second},
		GitHub:       GitHubConfig{TokenFrom: "gh"},
		Daemon:       DaemonConfig{Socket: "/tmp/ai-mux.sock"},
		Dashboard:    DashboardConfig{ItemsPerRepo: 3},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	for i, repo := range cfg.Repos {
		cfg.Repos[i].Path = expandHome(repo.Path)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if len(c.Repos) == 0 {
		return fmt.Errorf("at least one repo must be configured")
	}

	for i, repo := range c.Repos {
		if repo.Name == "" {
			return fmt.Errorf("repo at index %d has no name", i)
		}
		parts := strings.SplitN(repo.Name, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("repo %q must be in owner/repo format", repo.Name)
		}
		if repo.Path == "" {
			return fmt.Errorf("repo %q has no local path configured", repo.Name)
		}
	}

	if c.PollInterval.Duration <= 0 {
		return fmt.Errorf("poll_interval must be positive")
	}

	for i, agent := range c.Agents {
		if agent.Name == "" {
			return fmt.Errorf("agent at index %d has no name", i)
		}
		if agent.Command == "" {
			return fmt.Errorf("agent %q has no command", agent.Name)
		}
	}

	if c.DefaultAgent != "" {
		found := false
		for _, agent := range c.Agents {
			if agent.Name == c.DefaultAgent {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("default_agent %q not found in agents list", c.DefaultAgent)
		}
	}

	return nil
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
repos:
  - name: owner/repo-a
    path: /tmp/repo-a
  - name: org/repo-b
    path: /tmp/repo-b

pollInterval: 30s

github:
  tokenFrom: gh

agents:
  - name: claude
    command: claude

defaultAgent: claude

notifications:
  desktop:
    enabled: true
    events:
      - review_requested
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Name != "owner/repo-a" {
		t.Errorf("expected repo name owner/repo-a, got %s", cfg.Repos[0].Name)
	}
	if cfg.Repos[0].Path != "/tmp/repo-a" {
		t.Errorf("expected repo path /tmp/repo-a, got %s", cfg.Repos[0].Path)
	}
	if cfg.PollInterval.Duration != 30*time.Second {
		t.Errorf("expected 30s poll interval, got %s", cfg.PollInterval.Duration)
	}
	if cfg.GitHub.TokenFrom != "gh" {
		t.Errorf("expected tokenFrom gh, got %s", cfg.GitHub.TokenFrom)
	}
	if len(cfg.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(cfg.Agents))
	}
	if cfg.DefaultAgent != "claude" {
		t.Errorf("expected defaultAgent claude, got %s", cfg.DefaultAgent)
	}
	if !cfg.Notifications.Desktop.Enabled {
		t.Error("expected desktop notifications enabled")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "{{invalid yaml")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_PollIntervalParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"30s", 30 * time.Second},
		{"1m", time.Minute},
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			content := "repos:\n  - name: o/r\n    path: /tmp/r\npollInterval: " + tt.input
			path := writeTemp(t, content)
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.PollInterval.Duration != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, cfg.PollInterval.Duration)
			}
		})
	}
}

func TestLoad_HomeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	content := "repos:\n  - name: o/r\n    path: ~/projects/myrepo\npollInterval: 30s"
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(home, "projects/myrepo")
	if cfg.Repos[0].Path != expected {
		t.Errorf("expected %s, got %s", expected, cfg.Repos[0].Path)
	}
}

func TestValidate_MissingRepos(t *testing.T) {
	cfg := Default()
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing repos")
	}
}

func TestValidate_InvalidRepoFormat(t *testing.T) {
	cfg := Default()
	cfg.Repos = []RepoConfig{{Name: "noslash", Path: "/tmp"}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid repo format")
	}
}

func TestValidate_MissingRepoPath(t *testing.T) {
	cfg := Default()
	cfg.Repos = []RepoConfig{{Name: "owner/repo"}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing repo path")
	}
}

func TestValidate_InvalidPollInterval(t *testing.T) {
	cfg := Default()
	cfg.Repos = []RepoConfig{{Name: "o/r", Path: "/tmp"}}
	cfg.PollInterval = Duration{0}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for zero poll interval")
	}
}

func TestValidate_DefaultAgentNotFound(t *testing.T) {
	cfg := Default()
	cfg.Repos = []RepoConfig{{Name: "o/r", Path: "/tmp"}}
	cfg.DefaultAgent = "nonexistent"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for unknown defaultAgent")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := Default()
	cfg.Repos = []RepoConfig{{Name: "owner/repo", Path: "/tmp/repo"}}
	cfg.Agents = []AgentConfig{{Name: "claude", Command: "claude"}}
	cfg.DefaultAgent = "claude"
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.PollInterval.Duration != 30*time.Second {
		t.Errorf("expected 30s default poll interval, got %s", cfg.PollInterval.Duration)
	}
	if cfg.GitHub.TokenFrom != "gh" {
		t.Errorf("expected default tokenFrom gh, got %s", cfg.GitHub.TokenFrom)
	}
	if cfg.Daemon.Socket != "/tmp/ai-mux.sock" {
		t.Errorf("expected default socket /tmp/ai-mux.sock, got %s", cfg.Daemon.Socket)
	}
	if cfg.Dashboard.ItemsPerRepo != 3 {
		t.Errorf("expected default itemsPerRepo 3, got %d", cfg.Dashboard.ItemsPerRepo)
	}
}

func TestLoad_DashboardConfig(t *testing.T) {
	content := "repos:\n  - name: o/r\n    path: /tmp/r\npollInterval: 30s\ndashboard:\n  itemsPerRepo: 5"
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Dashboard.ItemsPerRepo != 5 {
		t.Errorf("expected itemsPerRepo 5, got %d", cfg.Dashboard.ItemsPerRepo)
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}

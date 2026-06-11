package session

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type TmuxExecutor interface {
	NewSession(name, workdir, command string) error
	KillSession(name string) error
	SendKeys(name, keys string) error
	TypeKeys(name, text string) error
	ListSessions(prefix string) ([]string, error)
	HasSession(name string) bool
	IsPaneDead(name string) bool
	PanePID(name string) (int, error)
	PipePaneToFile(name, path string) error
	CapturePane(name string) (string, error)
}

type tmuxCLI struct{}

func NewTmuxCLI() TmuxExecutor {
	return &tmuxCLI{}
}

func (t *tmuxCLI) NewSession(name, workdir, command string) error {
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", workdir, command)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux new-session: %s: %w", strings.TrimSpace(string(out)), err)
	}
	_ = exec.Command("tmux", "set-option", "-t", name, "remain-on-exit", "on").Run()
	return nil
}

func (t *tmuxCLI) KillSession(name string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux kill-session: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (t *tmuxCLI) SendKeys(name, keys string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", name, keys, "Enter")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (t *tmuxCLI) TypeKeys(name, text string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", name, "-l", text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux type-keys: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (t *tmuxCLI) ListSessions(prefix string) ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %w", err)
	}

	var matched []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			matched = append(matched, line)
		}
	}
	return matched, nil
}

func (t *tmuxCLI) HasSession(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

func (t *tmuxCLI) IsPaneDead(name string) bool {
	cmd := exec.Command("tmux", "display-message", "-t", name, "-p", "#{pane_dead}")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "1"
}

func (t *tmuxCLI) PanePID(name string) (int, error) {
	cmd := exec.Command("tmux", "display-message", "-t", name, "-p", "#{pane_pid}")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("tmux display-message: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parsing pane pid: %w", err)
	}
	return pid, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func (t *tmuxCLI) PipePaneToFile(name, path string) error {
	pipeCmd := "cat >> " + shellQuote(path)
	cmd := exec.Command("tmux", "pipe-pane", "-t", name, pipeCmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux pipe-pane: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (t *tmuxCLI) CapturePane(name string) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-t", name, "-p", "-e")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane: %w", err)
	}
	return string(out), nil
}

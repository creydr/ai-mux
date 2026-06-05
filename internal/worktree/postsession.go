package worktree

import (
	"fmt"
	"os/exec"
	"strings"
)

type PostSessionHandler interface {
	Handle(repoPath, wtPath, itemTitle string) error
}

type KeepHandler struct{}

func (h *KeepHandler) Handle(_, _, _ string) error {
	return nil
}

type AutoPRHandler struct {
	manager *Manager
}

func NewAutoPRHandler(manager *Manager) *AutoPRHandler {
	return &AutoPRHandler{manager: manager}
}

func (h *AutoPRHandler) Handle(repoPath, wtPath, itemTitle string) error {
	changed, err := h.manager.HasChanges(wtPath)
	if err != nil {
		return err
	}
	if !changed {
		return h.manager.Remove(repoPath, wtPath)
	}

	cmds := []struct {
		name string
		args []string
	}{
		{"git", []string{"add", "-A"}},
		{"git", []string{"commit", "-m", fmt.Sprintf("ai-mux: %s", itemTitle)}},
		{"git", []string{"push", "-u", "origin", "HEAD"}},
	}

	for _, c := range cmds {
		cmd := exec.Command(c.name, c.args...)
		cmd.Dir = wtPath
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("running %s %s: %s: %w", c.name, strings.Join(c.args, " "), strings.TrimSpace(string(out)), err)
		}
	}

	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = wtPath
	branchOut, err := branchCmd.Output()
	if err != nil {
		return fmt.Errorf("getting branch name: %w", err)
	}
	branch := strings.TrimSpace(string(branchOut))

	prCmd := exec.Command("gh", "pr", "create", "--draft",
		"--title", itemTitle,
		"--body", "Created by ai-mux",
		"--head", branch,
	)
	prCmd.Dir = wtPath
	if out, err := prCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating PR: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return h.manager.Remove(repoPath, wtPath)
}

func NewPostSessionHandler(behavior string, manager *Manager) PostSessionHandler {
	switch behavior {
	case "auto-pr":
		return NewAutoPRHandler(manager)
	default:
		return &KeepHandler{}
	}
}

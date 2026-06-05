package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const worktreeDir = ".worktrees"

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Create(repoPath, name string) (string, error) {
	wtDir := filepath.Join(repoPath, worktreeDir)
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		return "", fmt.Errorf("creating worktree directory: %w", err)
	}

	wtPath := filepath.Join(wtDir, name)

	cmd := exec.Command("git", "worktree", "add", "-b", "ai-mux/"+name, wtPath)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("creating worktree: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return wtPath, nil
}

func (m *Manager) Remove(repoPath, wtPath string) error {
	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("removing worktree: %s: %w", strings.TrimSpace(string(out)), err)
	}

	branchName := filepath.Base(wtPath)
	delCmd := exec.Command("git", "branch", "-D", "ai-mux/"+branchName)
	delCmd.Dir = repoPath
	delCmd.CombinedOutput()

	return nil
}

func (m *Manager) List(repoPath string) ([]string, error) {
	wtDir := filepath.Join(repoPath, worktreeDir)
	entries, err := os.ReadDir(wtDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			paths = append(paths, filepath.Join(wtDir, e.Name()))
		}
	}
	return paths, nil
}

func (m *Manager) HasChanges(wtPath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = wtPath
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("checking changes: %w", err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

func (m *Manager) Cleanup(repoPath string) (int, error) {
	worktrees, err := m.List(repoPath)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, wt := range worktrees {
		changed, err := m.HasChanges(wt)
		if err != nil {
			continue
		}
		if !changed {
			if err := m.Remove(repoPath, wt); err != nil {
				continue
			}
			removed++
		}
	}
	return removed, nil
}

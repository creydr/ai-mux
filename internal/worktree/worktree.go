package worktree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const worktreeDir = ".worktrees"

var ErrWorktreeExists = errors.New("worktree already exists")

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Exists(repoPath, name string) bool {
	wtPath := filepath.Join(repoPath, worktreeDir, name)
	info, err := os.Stat(wtPath)
	return err == nil && info.IsDir()
}

func (m *Manager) Create(repoPath, name string) (string, error) {
	wtDir := filepath.Join(repoPath, worktreeDir)
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		return "", fmt.Errorf("creating worktree directory: %w", err)
	}

	candidate := name
	for i := 2; ; i++ {
		wtPath := filepath.Join(wtDir, candidate)
		cmd := exec.Command("git", "worktree", "add", "-b", "ai-mux/"+candidate, wtPath)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			errStr := strings.TrimSpace(string(out))
			if strings.Contains(errStr, "already exists") {
				candidate = fmt.Sprintf("%s-%d", name, i)
				continue
			}
			return "", fmt.Errorf("creating worktree: %s: %w", errStr, err)
		}
		return wtPath, nil
	}
}

// CreateForPR creates a detached worktree and uses `gh pr checkout` to
// check out the PR's branch. This handles forks and remote branches that
// may not exist in any local remote. It first tries without --branch so
// that `gh pr view` can resolve the PR from the branch name. If the branch
// is already checked out elsewhere, it falls back to a renamed branch.
func (m *Manager) CreateForPR(repoPath, name, repoFullName string, prNumber int) (string, error) {
	wtDir := filepath.Join(repoPath, worktreeDir)
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		return "", fmt.Errorf("creating worktree directory: %w", err)
	}

	candidate := name
	for i := 2; ; i++ {
		wtPath := filepath.Join(wtDir, candidate)
		cmd := exec.Command("git", "worktree", "add", "--detach", wtPath)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			errStr := strings.TrimSpace(string(out))
			if strings.Contains(errStr, "already exists") {
				candidate = fmt.Sprintf("%s-%d", name, i)
				continue
			}
			return "", fmt.Errorf("creating worktree: %s: %w", errStr, err)
		}

		prNum := fmt.Sprintf("%d", prNumber)
		ghCmd := exec.Command("gh", "pr", "checkout", prNum, "--repo", repoFullName)
		ghCmd.Dir = wtPath
		if out, err := ghCmd.CombinedOutput(); err != nil {
			errStr := strings.TrimSpace(string(out))
			if strings.Contains(errStr, "already used by worktree") || strings.Contains(errStr, "already exists") {
				// Branch is checked out elsewhere — retry with a unique branch name
				branchName := fmt.Sprintf("ai-mux/%s", candidate)
				ghRetry := exec.Command("gh", "pr", "checkout", prNum, "--repo", repoFullName, "--branch", branchName)
				ghRetry.Dir = wtPath
				if out2, err2 := ghRetry.CombinedOutput(); err2 != nil {
					rmCmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
					rmCmd.Dir = repoPath
					rmCmd.Run()
					return "", fmt.Errorf("checking out PR #%s: %s: %w", prNum, strings.TrimSpace(string(out2)), err2)
				}
				return wtPath, nil
			}
			rmCmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
			rmCmd.Dir = repoPath
			rmCmd.Run()
			return "", fmt.Errorf("checking out PR #%s: %s: %w", prNum, errStr, err)
		}

		return wtPath, nil
	}
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

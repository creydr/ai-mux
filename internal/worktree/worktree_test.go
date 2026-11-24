package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args[1:], out, err)
		}
	}

	f, _ := os.Create(filepath.Join(dir, "README.md"))
	f.WriteString("# test\n")
	f.Close()

	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	cmd.CombinedOutput()

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = dir
	cmd.CombinedOutput()

	return dir
}

func TestManager_CreateAndRemove(t *testing.T) {
	repo := initRepo(t)
	mgr := NewManager()

	wtPath, err := mgr.Create(repo, "fix-bug-1")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatal("worktree directory should exist")
	}

	if err := mgr.Remove(repo, wtPath); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}
}

func TestManager_List(t *testing.T) {
	repo := initRepo(t)
	mgr := NewManager()

	mgr.Create(repo, "wt-1")
	mgr.Create(repo, "wt-2")

	list, err := mgr.List(repo)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(list))
	}
}

func TestManager_List_EmptyRepo(t *testing.T) {
	repo := initRepo(t)
	mgr := NewManager()

	list, err := mgr.List(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(list))
	}
}

func TestManager_HasChanges_NoChanges(t *testing.T) {
	repo := initRepo(t)
	mgr := NewManager()

	wtPath, _ := mgr.Create(repo, "clean")

	changed, err := mgr.HasChanges(wtPath)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("new worktree should have no changes")
	}
}

func TestManager_HasChanges_WithChanges(t *testing.T) {
	repo := initRepo(t)
	mgr := NewManager()

	wtPath, _ := mgr.Create(repo, "dirty")

	os.WriteFile(filepath.Join(wtPath, "new-file.txt"), []byte("hello"), 0644)

	changed, err := mgr.HasChanges(wtPath)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("worktree with new file should have changes")
	}
}

func TestFilterGitInfoMessages(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "filters Previous HEAD position",
			input:  "Previous HEAD position was cb0a3d27 fix(e2e): improve gateway test reliability (#572)\nfatal: could not read Username",
			expect: "fatal: could not read Username",
		},
		{
			name:   "filters Switched to",
			input:  "Switched to branch 'main'\nfatal: some error",
			expect: "fatal: some error",
		},
		{
			name:   "filters HEAD is now at",
			input:  "HEAD is now at abc1234 some commit\nerror: something broke",
			expect: "error: something broke",
		},
		{
			name:   "filters Already on",
			input:  "Already on 'main'\nerror: failed",
			expect: "error: failed",
		},
		{
			name:   "returns empty when only info messages",
			input:  "Previous HEAD position was abc1234 some commit\nSwitched to branch 'feature'",
			expect: "",
		},
		{
			name:   "preserves actual errors",
			input:  "fatal: bad revision 'nonexistent'",
			expect: "fatal: bad revision 'nonexistent'",
		},
		{
			name:   "handles empty input",
			input:  "",
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterGitInfoMessages(tt.input)
			if got != tt.expect {
				t.Errorf("filterGitInfoMessages() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestManager_Cleanup(t *testing.T) {
	repo := initRepo(t)
	mgr := NewManager()

	cleanPath, _ := mgr.Create(repo, "clean-wt")
	dirtyPath, _ := mgr.Create(repo, "dirty-wt")

	os.WriteFile(filepath.Join(dirtyPath, "change.txt"), []byte("data"), 0644)

	removed, err := mgr.Cleanup(repo)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	if _, err := os.Stat(cleanPath); !os.IsNotExist(err) {
		t.Error("clean worktree should be removed")
	}
	if _, err := os.Stat(dirtyPath); os.IsNotExist(err) {
		t.Error("dirty worktree should remain")
	}
}

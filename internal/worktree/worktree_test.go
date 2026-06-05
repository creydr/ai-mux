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

func TestNewPostSessionHandler(t *testing.T) {
	mgr := NewManager()

	h := NewPostSessionHandler("keep", mgr)
	if _, ok := h.(*KeepHandler); !ok {
		t.Error("expected KeepHandler for 'keep'")
	}

	h = NewPostSessionHandler("auto-pr", mgr)
	if _, ok := h.(*AutoPRHandler); !ok {
		t.Error("expected AutoPRHandler for 'auto-pr'")
	}

	h = NewPostSessionHandler("", mgr)
	if _, ok := h.(*KeepHandler); !ok {
		t.Error("expected KeepHandler for empty string")
	}
}

func TestKeepHandler(t *testing.T) {
	h := &KeepHandler{}
	if err := h.Handle("", "", ""); err != nil {
		t.Errorf("KeepHandler should always succeed, got %v", err)
	}
}

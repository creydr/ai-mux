package jsonfile

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/store"
)

func TestStore_SetGetItemState(t *testing.T) {
	s := newTestStore(t)

	state := store.ItemState{
		ItemID:     "item-1",
		Read:       false,
		LastSeenAt: time.Now().Truncate(time.Second),
	}

	if err := s.SetItemState(state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := s.GetItemState("item-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected item state, got nil")
	}
	if got.ItemID != "item-1" {
		t.Errorf("expected item-1, got %s", got.ItemID)
	}
	if got.Read {
		t.Error("expected unread")
	}
}

func TestStore_GetItemState_NotFound(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetItemState("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestStore_MarkRead(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetItemState(store.ItemState{ItemID: "item-1"}); err != nil {
		t.Fatal(err)
	}

	if err := s.MarkRead("item-1"); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetItemState("item-1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Read {
		t.Error("expected item to be marked as read")
	}
}

func TestStore_MarkRead_NewItem(t *testing.T) {
	s := newTestStore(t)

	if err := s.MarkRead("new-item"); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetItemState("new-item")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected item state to be created")
	}
	if !got.Read {
		t.Error("expected item to be marked as read")
	}
}

func TestStore_ListItemStates(t *testing.T) {
	s := newTestStore(t)

	for _, id := range []string{"a", "b", "c"} {
		if err := s.SetItemState(store.ItemState{ItemID: id}); err != nil {
			t.Fatal(err)
		}
	}

	states, err := s.ListItemStates()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 3 {
		t.Errorf("expected 3 states, got %d", len(states))
	}
}

func TestStore_PollTimes(t *testing.T) {
	s := newTestStore(t)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}
	now := time.Now().Truncate(time.Second)

	if err := s.SetLastPollTime(repo, now); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetLastPollTime(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(now) {
		t.Errorf("expected %v, got %v", now, got)
	}
}

func TestStore_PollTimes_NotFound(t *testing.T) {
	s := newTestStore(t)
	repo := provider.RepoRef{Owner: "o", Repo: "r"}

	got, err := s.GetLastPollTime(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time, got %v", got)
	}
}

func TestStore_Sessions(t *testing.T) {
	s := newTestStore(t)

	session := store.SessionState{
		ID:        "sess-1",
		ItemID:    "item-1",
		AgentName: "claude",
		StartedAt: time.Now().Truncate(time.Second),
		Status:    "running",
	}

	if err := s.SaveSession(session); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetSession("sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.AgentName != "claude" {
		t.Errorf("expected claude, got %s", got.AgentName)
	}

	sessions, err := s.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
}

func TestStore_Worktrees(t *testing.T) {
	s := newTestStore(t)

	wt := store.WorktreeState{
		Path:      "/tmp/wt",
		Repo:      "owner/repo",
		Action:    "fix_issue",
		SessionID: "sess-1",
		CreatedAt: time.Now().Truncate(time.Second),
	}

	if err := s.SaveWorktree(wt); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetWorktree("/tmp/wt")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected worktree, got nil")
	}
	if got.Action != "fix_issue" {
		t.Errorf("expected fix_issue, got %s", got.Action)
	}

	worktrees, err := s.ListWorktrees()
	if err != nil {
		t.Fatal(err)
	}
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}

	if err := s.RemoveWorktree("/tmp/wt"); err != nil {
		t.Fatal(err)
	}

	worktrees, err = s.ListWorktrees()
	if err != nil {
		t.Fatal(err)
	}
	if len(worktrees) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(worktrees))
	}
}

func TestStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s1, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.SetItemState(store.ItemState{ItemID: "item-1", Read: true}); err != nil {
		t.Fatal(err)
	}

	s2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.GetItemState("item-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected persisted item state")
	}
	if !got.Read {
		t.Error("expected item to be read after reload")
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := newTestStore(t)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "item-" + string(rune('a'+n))
			_ = s.SetItemState(store.ItemState{ItemID: id})
			_, _ = s.GetItemState(id)
			_ = s.MarkRead(id)
		}(i)
	}
	wg.Wait()
}

func TestStore_CreateDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "state.json")

	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetItemState(store.ItemState{ItemID: "item-1"}); err != nil {
		t.Fatal(err)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, err := New(path)
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	return s
}

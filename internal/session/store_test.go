package session

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	sessions := map[string]*Session{
		"fix-42-abcd": {
			ID:         "fix-42-abcd",
			ItemRepo:   "owner/repo",
			ItemNumber: 42,
			ItemType:   "issue",
			Agent:      "claude",
			Status:     StatusRunning,
			CreatedAt:  time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC),
		},
		"rev-15-efgh": {
			ID:         "rev-15-efgh",
			ItemRepo:   "owner/repo",
			ItemNumber: 15,
			ItemType:   "pr",
			Agent:      "claude",
			Status:     StatusCompleted,
			CreatedAt:  time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC),
		},
	}

	if err := store.Save(sessions); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(loaded))
	}

	s := loaded["fix-42-abcd"]
	if s == nil {
		t.Fatal("session fix-42-abcd not found")
	}
	if s.ItemNumber != 42 {
		t.Errorf("ItemNumber = %d, want 42", s.ItemNumber)
	}
	if s.Status != StatusRunning {
		t.Errorf("Status = %q, want %q", s.Status, StatusRunning)
	}
}

func TestStore_LoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "nonexistent"))

	sessions, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected empty map, got %d sessions", len(sessions))
	}
}

package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/store"
)

type fileState struct {
	Items     map[string]store.ItemState     `json:"items"`
	PollTimes map[string]time.Time           `json:"poll_times"`
	Sessions  map[string]store.SessionState  `json:"sessions"`
	Worktrees map[string]store.WorktreeState `json:"worktrees"`
}

type Store struct {
	path  string
	mu    sync.RWMutex
	state fileState
}

func New(path string) (*Store, error) {
	s := &Store{
		path: path,
		state: fileState{
			Items:     make(map[string]store.ItemState),
			PollTimes: make(map[string]time.Time),
			Sessions:  make(map[string]store.SessionState),
			Worktrees: make(map[string]store.WorktreeState),
		},
	}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &s.state); err != nil {
			return nil, fmt.Errorf("parsing state file: %w", err)
		}
		if s.state.Items == nil {
			s.state.Items = make(map[string]store.ItemState)
		}
		if s.state.PollTimes == nil {
			s.state.PollTimes = make(map[string]time.Time)
		}
		if s.state.Sessions == nil {
			s.state.Sessions = make(map[string]store.SessionState)
		}
		if s.state.Worktrees == nil {
			s.state.Worktrees = make(map[string]store.WorktreeState)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	return s, nil
}

func (s *Store) GetItemState(itemID string) (*store.ItemState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.state.Items[itemID]
	if !ok {
		return nil, nil
	}
	return &state, nil
}

func (s *Store) SetItemState(state store.ItemState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Items[state.ItemID] = state
	return s.flush()
}

func (s *Store) ListItemStates() ([]store.ItemState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	states := make([]store.ItemState, 0, len(s.state.Items))
	for _, state := range s.state.Items {
		states = append(states, state)
	}
	return states, nil
}

func (s *Store) MarkRead(itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.state.Items[itemID]
	if !ok {
		state = store.ItemState{ItemID: itemID}
	}
	state.Read = true
	s.state.Items[itemID] = state
	return s.flush()
}

func (s *Store) GetLastPollTime(repo provider.RepoRef) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.state.PollTimes[repo.String()]
	if !ok {
		return time.Time{}, nil
	}
	return t, nil
}

func (s *Store) SetLastPollTime(repo provider.RepoRef, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.PollTimes[repo.String()] = t
	return s.flush()
}

func (s *Store) SaveSession(session store.SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Sessions[session.ID] = session
	return s.flush()
}

func (s *Store) GetSession(id string) (*store.SessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.state.Sessions[id]
	if !ok {
		return nil, nil
	}
	return &session, nil
}

func (s *Store) ListSessions() ([]store.SessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]store.SessionState, 0, len(s.state.Sessions))
	for _, session := range s.state.Sessions {
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s *Store) SaveWorktree(wt store.WorktreeState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Worktrees[wt.Path] = wt
	return s.flush()
}

func (s *Store) GetWorktree(path string) (*store.WorktreeState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wt, ok := s.state.Worktrees[path]
	if !ok {
		return nil, nil
	}
	return &wt, nil
}

func (s *Store) ListWorktrees() ([]store.WorktreeState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	worktrees := make([]store.WorktreeState, 0, len(s.state.Worktrees))
	for _, wt := range s.state.Worktrees {
		worktrees = append(worktrees, wt)
	}
	return worktrees, nil
}

func (s *Store) RemoveWorktree(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.state.Worktrees, path)
	return s.flush()
}

func (s *Store) Close() error {
	return nil
}

func (s *Store) flush() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing temp state file: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("renaming state file: %w", err)
	}

	return nil
}

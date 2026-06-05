package store

import (
	"time"

	"github.com/creydr/ai-mux/internal/provider"
)

type ItemState struct {
	ItemID     string    `json:"item_id"`
	Read       bool      `json:"read"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type SessionState struct {
	ID        string    `json:"id"`
	ItemID    string    `json:"item_id"`
	AgentName string    `json:"agent_name"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"`
}

type WorktreeState struct {
	Path      string    `json:"path"`
	Repo      string    `json:"repo"`
	Action    string    `json:"action"`
	SessionID string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Store interface {
	GetItemState(itemID string) (*ItemState, error)
	SetItemState(state ItemState) error
	ListItemStates() ([]ItemState, error)
	MarkRead(itemID string) error

	GetLastPollTime(repo provider.RepoRef) (time.Time, error)
	SetLastPollTime(repo provider.RepoRef, t time.Time) error

	SaveSession(session SessionState) error
	GetSession(id string) (*SessionState, error)
	ListSessions() ([]SessionState, error)

	SaveWorktree(wt WorktreeState) error
	GetWorktree(path string) (*WorktreeState, error)
	ListWorktrees() ([]WorktreeState, error)
	RemoveWorktree(path string) error

	Close() error
}

package session

import (
	"crypto/rand"
	"fmt"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusStopped   Status = "stopped"
)

type Session struct {
	ID           string     `json:"id"`
	Name         string     `json:"name,omitempty"`
	ItemRepo     string     `json:"item_repo"`
	ItemNumber   int        `json:"item_number"`
	ItemType     string     `json:"item_type"`
	Agent        string     `json:"agent"`
	TmuxSession  string     `json:"tmux_session"`
	Worktree     string     `json:"worktree"`
	RepoPath     string     `json:"repo_path"`
	Status       Status     `json:"status"`
	WaitingInput bool       `json:"waiting_input"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ExitCode     *int       `json:"exit_code,omitempty"`
	Error        string     `json:"error,omitempty"`
}

func (s *Session) IsActive() bool {
	return s.Status == StatusPending || s.Status == StatusRunning
}

func generateID(prefix string, number int) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s-%d-%x", prefix, number, b)
}

package dashboard

import (
	"time"

	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

type itemsReceivedMsg struct {
	issues []provider.Item
	prs    []provider.Item
}

type eventReceivedMsg struct {
	event event.Event
}

type errMsg struct {
	err error
}

type connectResultMsg struct {
	err error
}

type repoExpandedMsg struct {
	repo           string
	items          []provider.Item
	itemType       provider.ItemType
	requestedLimit int
}

type sessionsReceivedMsg struct {
	sessions []protocol.SessionPayload
}

type sessionSpawnedMsg struct {
	session protocol.SessionPayload
}

type sessionStoppedMsg struct {
	sessionID string
}

type statusMsg struct {
	text string
}

type statusTickMsg struct {
	id  int
	due time.Time
}

type sessionOutputMsg struct {
	sessionID string
	data      string
}

type sessionAttachedMsg struct {
	session protocol.SessionPayload
}

type attachNonOutputMsg struct{}

type tmuxDetachedMsg struct {
	err error
}

type worktreeExistsMsg struct {
	repo     string
	number   int
	itemType string
	agent    string
}

type sessionRenamedMsg struct {
	sessionID string
	name      string
}

type sessionTickMsg struct {
	id int
}

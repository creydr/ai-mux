package attach

import (
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

type itemLoadedMsg struct {
	item *provider.Item
}

type reviewsLoadedMsg struct {
	reviews []provider.Review
}

type commentsLoadedMsg struct {
	comments []provider.Comment
}

type contentRenderedMsg struct {
	lines []string
}

type CloseMsg struct{}

type SpawnSessionMsg struct {
	Ref Ref
}

type AttachSessionMsg struct {
	SessionID string
	Name      string
	Status    string
}

type sessionsLoadedMsg struct {
	sessions []protocol.SessionPayload
}

type statusTextMsg struct {
	text string
}

type jiraItemLoadedMsg struct {
	item     *provider.JiraItem
	comments []provider.JiraComment
}

type SpawnJiraSessionMsg struct {
	Key string
}

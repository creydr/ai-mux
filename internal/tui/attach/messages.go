package attach

import (
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

type errMsg struct {
	err error
}

type contentRenderedMsg struct {
	lines []string
}

type CloseMsg struct{}

type SpawnSessionMsg struct {
	Ref Ref
}

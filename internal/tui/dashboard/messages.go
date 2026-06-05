package dashboard

import (
	"github.com/creydr/ai-mux/internal/event"
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

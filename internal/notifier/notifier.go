package notifier

import (
	"github.com/creydr/ai-mux/internal/event"
)

type Notifier interface {
	Name() string
	Notify(ev event.Event) error
	ShouldNotify(ev event.Event) bool
	Close() error
}

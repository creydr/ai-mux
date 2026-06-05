package tui

import (
	"sync"

	"github.com/creydr/ai-mux/internal/event"
)

type Notifier struct {
	mu     sync.RWMutex
	counts map[event.Type]int
}

func New() *Notifier {
	return &Notifier{
		counts: make(map[event.Type]int),
	}
}

func (n *Notifier) Name() string {
	return "tui"
}

func (n *Notifier) ShouldNotify(ev event.Event) bool {
	switch ev.Type {
	case event.TypeNewIssue, event.TypeNewPR, event.TypeReviewReceived, event.TypeNewComment:
		return true
	}
	return false
}

func (n *Notifier) Notify(ev event.Event) error {
	if !n.ShouldNotify(ev) {
		return nil
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.counts[ev.Type]++
	return nil
}

func (n *Notifier) Count(evType event.Type) int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.counts[evType]
}

func (n *Notifier) TotalCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	total := 0
	for _, c := range n.counts {
		total += c
	}
	return total
}

func (n *Notifier) Clear(evType event.Type) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.counts, evType)
}

func (n *Notifier) ClearAll() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.counts = make(map[event.Type]int)
}

func (n *Notifier) Close() error {
	return nil
}

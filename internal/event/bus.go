package event

import (
	"log"
	"sync"
)

const subscriberBufferSize = 64

type subscription struct {
	ch     chan Event
	types  map[Type]bool
	closed bool
}

type Bus struct {
	mu          sync.RWMutex
	subscribers []*subscription
	done        bool
}

func NewBus() *Bus {
	return &Bus{}
}

func (b *Bus) Subscribe(types ...Type) <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	typeSet := make(map[Type]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}

	sub := &subscription{
		ch:    make(chan Event, subscriberBufferSize),
		types: typeSet,
	}
	b.subscribers = append(b.subscribers, sub)
	return sub.ch
}

func (b *Bus) Unsubscribe(ch <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, sub := range b.subscribers {
		if sub.ch == ch {
			sub.closed = true
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			return
		}
	}
}

func (b *Bus) Publish(ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.done {
		return
	}

	for _, sub := range b.subscribers {
		if sub.closed {
			continue
		}
		if len(sub.types) > 0 && !sub.types[ev.Type] {
			continue
		}
		select {
		case sub.ch <- ev:
		default:
			log.Printf("warning: dropping event %s for slow subscriber", ev.Type)
		}
	}
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.done = true
	for _, sub := range b.subscribers {
		if !sub.closed {
			close(sub.ch)
			sub.closed = true
		}
	}
	b.subscribers = nil
}

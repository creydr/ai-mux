package event

import (
	"testing"
	"time"

	"github.com/creydr/ai-mux/internal/provider"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	ch := bus.Subscribe()
	ev := Event{Type: TypeNewIssue, Item: &provider.Item{Title: "test"}, Timestamp: time.Now()}
	bus.Publish(ev)

	select {
	case received := <-ch:
		if received.Type != TypeNewIssue {
			t.Errorf("expected TypeNewIssue, got %s", received.Type)
		}
		if received.Item.Title != "test" {
			t.Errorf("expected title test, got %s", received.Item.Title)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBus_FilteredSubscription(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	ch := bus.Subscribe(TypeNewIssue)

	bus.Publish(Event{Type: TypeNewPR, Timestamp: time.Now()})
	bus.Publish(Event{Type: TypeNewIssue, Item: &provider.Item{Title: "issue"}, Timestamp: time.Now()})

	select {
	case received := <-ch:
		if received.Type != TypeNewIssue {
			t.Errorf("expected TypeNewIssue, got %s", received.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for filtered event")
	}

	select {
	case ev := <-ch:
		t.Fatalf("should not have received PR event, got %s", ev.Type)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	ch1 := bus.Subscribe()
	ch2 := bus.Subscribe()

	bus.Publish(Event{Type: TypeNewIssue, Timestamp: time.Now()})

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case received := <-ch:
			if received.Type != TypeNewIssue {
				t.Errorf("subscriber %d: expected TypeNewIssue, got %s", i, received.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	ch := bus.Subscribe()
	bus.Unsubscribe(ch)

	bus.Publish(Event{Type: TypeNewIssue, Timestamp: time.Now()})

	select {
	case <-ch:
		t.Error("should not receive events after unsubscribe")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBus_ConcurrentPublishUnsubscribe(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			ch := bus.Subscribe()
			bus.Publish(Event{Type: TypeNewIssue, Timestamp: time.Now()})
			bus.Unsubscribe(ch)
		}
	}()

	for i := 0; i < 1000; i++ {
		bus.Publish(Event{Type: TypeNewPR, Timestamp: time.Now()})
	}

	<-done
}

func TestBus_Close(t *testing.T) {
	bus := NewBus()

	ch1 := bus.Subscribe()
	ch2 := bus.Subscribe()

	bus.Close()

	for i, ch := range []<-chan Event{ch1, ch2} {
		_, ok := <-ch
		if ok {
			t.Errorf("subscriber %d: expected channel to be closed", i)
		}
	}
}

func TestBus_NonBlocking(t *testing.T) {
	bus := NewBus()
	defer bus.Close()

	_ = bus.Subscribe()

	for i := 0; i < subscriberBufferSize+10; i++ {
		bus.Publish(Event{Type: TypeNewIssue, Timestamp: time.Now()})
	}
}

func TestBus_PublishAfterClose(t *testing.T) {
	bus := NewBus()
	bus.Close()

	bus.Publish(Event{Type: TypeNewIssue, Timestamp: time.Now()})
}

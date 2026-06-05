package tui

import (
	"testing"

	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/provider"
)

func TestNotifier_Notify_NewIssue(t *testing.T) {
	n := New()
	ev := event.Event{
		Type: event.TypeNewIssue,
		Item: &provider.Item{Number: 1, Title: "Bug"},
	}

	n.Notify(ev)

	if n.Count(event.TypeNewIssue) != 1 {
		t.Errorf("expected count 1, got %d", n.Count(event.TypeNewIssue))
	}
}

func TestNotifier_Notify_IgnoresUpdates(t *testing.T) {
	n := New()
	n.Notify(event.Event{Type: event.TypeIssueUpdated})

	if n.Count(event.TypeIssueUpdated) != 0 {
		t.Error("should not count update events")
	}
}

func TestNotifier_TotalCount(t *testing.T) {
	n := New()
	n.Notify(event.Event{Type: event.TypeNewIssue, Item: &provider.Item{}})
	n.Notify(event.Event{Type: event.TypeNewPR, Item: &provider.Item{}})
	n.Notify(event.Event{Type: event.TypeNewIssue, Item: &provider.Item{}})

	if n.TotalCount() != 3 {
		t.Errorf("expected total 3, got %d", n.TotalCount())
	}
}

func TestNotifier_Clear(t *testing.T) {
	n := New()
	n.Notify(event.Event{Type: event.TypeNewIssue, Item: &provider.Item{}})
	n.Notify(event.Event{Type: event.TypeNewPR, Item: &provider.Item{}})

	n.Clear(event.TypeNewIssue)

	if n.Count(event.TypeNewIssue) != 0 {
		t.Error("issue count should be cleared")
	}
	if n.Count(event.TypeNewPR) != 1 {
		t.Error("PR count should remain")
	}
}

func TestNotifier_ClearAll(t *testing.T) {
	n := New()
	n.Notify(event.Event{Type: event.TypeNewIssue, Item: &provider.Item{}})
	n.Notify(event.Event{Type: event.TypeNewPR, Item: &provider.Item{}})

	n.ClearAll()

	if n.TotalCount() != 0 {
		t.Error("total count should be 0 after clear all")
	}
}

func TestNotifier_Name(t *testing.T) {
	n := New()
	if n.Name() != "tui" {
		t.Errorf("expected 'tui', got %q", n.Name())
	}
}

func TestNotifier_ShouldNotify(t *testing.T) {
	n := New()

	if !n.ShouldNotify(event.Event{Type: event.TypeNewIssue}) {
		t.Error("should notify for new issues")
	}
	if !n.ShouldNotify(event.Event{Type: event.TypeNewPR}) {
		t.Error("should notify for new PRs")
	}
	if !n.ShouldNotify(event.Event{Type: event.TypeReviewReceived}) {
		t.Error("should notify for reviews")
	}
	if n.ShouldNotify(event.Event{Type: event.TypeIssueUpdated}) {
		t.Error("should not notify for updates")
	}
}

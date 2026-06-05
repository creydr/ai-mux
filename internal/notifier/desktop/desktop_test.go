package desktop

import (
	"testing"

	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/provider"
)

func TestNotifier_ShouldNotify_Disabled(t *testing.T) {
	n := New(false, nil)
	ev := event.Event{Type: event.TypeNewIssue}
	if n.ShouldNotify(ev) {
		t.Error("disabled notifier should not notify")
	}
}

func TestNotifier_ShouldNotify_AllEvents(t *testing.T) {
	n := New(true, nil)
	ev := event.Event{Type: event.TypeNewIssue}
	if !n.ShouldNotify(ev) {
		t.Error("should notify for any event when no filter set")
	}
}

func TestNotifier_ShouldNotify_FilteredEvents(t *testing.T) {
	n := New(true, []string{"new_issue", "new_pr"})

	if !n.ShouldNotify(event.Event{Type: event.TypeNewIssue}) {
		t.Error("should notify for new_issue")
	}
	if !n.ShouldNotify(event.Event{Type: event.TypeNewPR}) {
		t.Error("should notify for new_pr")
	}
	if n.ShouldNotify(event.Event{Type: event.TypeIssueUpdated}) {
		t.Error("should not notify for issue_updated")
	}
}

func TestNotifier_Name(t *testing.T) {
	n := New(true, nil)
	if n.Name() != "desktop" {
		t.Errorf("expected 'desktop', got %q", n.Name())
	}
}

func TestFormatNotification_NewIssue(t *testing.T) {
	title, body := formatNotification(event.Event{
		Type: event.TypeNewIssue,
		Item: &provider.Item{Number: 42, Title: "Bug"},
	})
	if title != "New Issue" {
		t.Errorf("expected 'New Issue', got %q", title)
	}
	if body != "#42: Bug" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestFormatNotification_Review(t *testing.T) {
	title, body := formatNotification(event.Event{
		Type:   event.TypeReviewReceived,
		Review: &provider.Review{Author: "reviewer", State: "APPROVED"},
	})
	if title != "Review Received" {
		t.Errorf("expected 'Review Received', got %q", title)
	}
	if body != "reviewer: APPROVED" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestBuildCommand(t *testing.T) {
	cmd := BuildCommand("Test Title", "Test Body")
	if cmd.Args[0] != "notify-send" {
		t.Errorf("expected notify-send, got %q", cmd.Args[0])
	}
	if cmd.Args[len(cmd.Args)-2] != "Test Title" {
		t.Errorf("expected title in args: %v", cmd.Args)
	}
}

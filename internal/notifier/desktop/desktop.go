package desktop

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/creydr/ai-mux/internal/event"
)

type Notifier struct {
	enabled    bool
	eventTypes map[event.Type]bool
}

func New(enabled bool, events []string) *Notifier {
	types := make(map[event.Type]bool, len(events))
	for _, e := range events {
		types[event.Type(e)] = true
	}
	return &Notifier{
		enabled:    enabled,
		eventTypes: types,
	}
}

func (n *Notifier) Name() string {
	return "desktop"
}

func (n *Notifier) ShouldNotify(ev event.Event) bool {
	if !n.enabled {
		return false
	}
	if len(n.eventTypes) == 0 {
		return true
	}
	return n.eventTypes[ev.Type]
}

func (n *Notifier) Notify(ev event.Event) error {
	if !n.ShouldNotify(ev) {
		return nil
	}

	title, body := formatNotification(ev)
	return sendNotification(title, body)
}

func (n *Notifier) Close() error {
	return nil
}

func formatNotification(ev event.Event) (string, string) {
	switch ev.Type {
	case event.TypeNewIssue:
		if ev.Item != nil {
			return "New Issue", fmt.Sprintf("#%d: %s", ev.Item.Number, ev.Item.Title)
		}
	case event.TypeNewPR:
		if ev.Item != nil {
			return "New PR", fmt.Sprintf("#%d: %s", ev.Item.Number, ev.Item.Title)
		}
	case event.TypeIssueUpdated:
		if ev.Item != nil {
			return "Issue Updated", fmt.Sprintf("#%d: %s", ev.Item.Number, ev.Item.Title)
		}
	case event.TypePRUpdated:
		if ev.Item != nil {
			return "PR Updated", fmt.Sprintf("#%d: %s", ev.Item.Number, ev.Item.Title)
		}
	case event.TypeReviewReceived:
		if ev.Review != nil {
			return "Review Received", fmt.Sprintf("%s: %s", ev.Review.Author, ev.Review.State)
		}
	case event.TypeNewComment:
		if ev.Comment != nil {
			return "New Comment", fmt.Sprintf("%s: %s", ev.Comment.Author, ev.Comment.Body)
		}
	}
	return "ai-mux", string(ev.Type)
}

func sendNotification(title, body string) error {
	cmd := buildCommand(title, body)
	return cmd.Run()
}

func buildCommand(title, body string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		return exec.Command("osascript", "-e", script)
	default:
		return exec.Command("notify-send", "--app-name=ai-mux", title, body)
	}
}

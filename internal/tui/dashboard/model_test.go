package dashboard

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/provider"
)

func testItems() ([]provider.Item, []provider.Item) {
	issues := []provider.Item{
		{ID: "o/r/issues/1", Number: 1, Title: "Bug report", Type: provider.ItemTypeIssue, Repo: provider.RepoRef{Owner: "o", Repo: "r"}, URL: "https://github.com/o/r/issues/1"},
		{ID: "o/r/issues/2", Number: 2, Title: "Feature request", Type: provider.ItemTypeIssue, Repo: provider.RepoRef{Owner: "o", Repo: "r"}},
	}
	prs := []provider.Item{
		{ID: "o/r/prs/10", Number: 10, Title: "Fix typo", Type: provider.ItemTypePR, Repo: provider.RepoRef{Owner: "o", Repo: "r"}, URL: "https://github.com/o/r/pull/10"},
	}
	return issues, prs
}

func TestModel_InitWithoutConn(t *testing.T) {
	m := New(nil)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil without a connection")
	}
}

func TestModel_TabSwitch(t *testing.T) {
	m := New(nil)
	issues, prs := testItems()
	m.issues = issues
	m.prs = prs

	if m.activeTab != tabIssues {
		t.Fatal("should start on issues tab")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	if m.activeTab != tabPRs {
		t.Error("should switch to PRs tab")
	}
	if m.cursor != 0 {
		t.Error("cursor should reset on tab switch")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	if m.activeTab != tabIssues {
		t.Error("should wrap back to issues tab")
	}
}

func TestModel_NavigateDown(t *testing.T) {
	m := New(nil)
	issues, _ := testItems()
	m.issues = issues

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = updated.(Model)

	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	m = updated.(Model)

	if m.cursor != 1 {
		t.Error("cursor should not go past last item")
	}
}

func TestModel_NavigateUp(t *testing.T) {
	m := New(nil)
	issues, _ := testItems()
	m.issues = issues
	m.cursor = 1

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'k'})
	m = updated.(Model)

	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	m = updated.(Model)

	if m.cursor != 0 {
		t.Error("cursor should not go below 0")
	}
}

func TestModel_NavigateWithArrowKeys(t *testing.T) {
	m := New(nil)
	issues, _ := testItems()
	m.issues = issues

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Error("down arrow should move cursor down")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = updated.(Model)
	if m.cursor != 0 {
		t.Error("up arrow should move cursor up")
	}
}

func TestModel_ItemsReceived(t *testing.T) {
	m := New(nil)
	issues, prs := testItems()

	updated, _ := m.Update(itemsReceivedMsg{issues: issues, prs: prs})
	m = updated.(Model)

	if len(m.issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(m.issues))
	}
	if len(m.prs) != 1 {
		t.Errorf("expected 1 PR, got %d", len(m.prs))
	}
	if m.cursor != 0 {
		t.Error("cursor should reset after items received")
	}
}

func TestModel_EventReceived_NewIssue(t *testing.T) {
	m := New(nil)
	m.activeTab = tabPRs

	updated, _ := m.Update(eventReceivedMsg{event: event.Event{
		Type: event.TypeNewIssue,
		Item: &provider.Item{ID: "o/r/issues/99", Number: 99, Title: "New bug"},
	}})
	m = updated.(Model)

	if len(m.issues) != 1 {
		t.Error("new issue should be added")
	}
	if m.issueBadge != 1 {
		t.Error("badge should increment when on different tab")
	}
}

func TestModel_EventReceived_NewPR(t *testing.T) {
	m := New(nil)
	m.activeTab = tabIssues

	updated, _ := m.Update(eventReceivedMsg{event: event.Event{
		Type: event.TypeNewPR,
		Item: &provider.Item{ID: "o/r/prs/50", Number: 50, Title: "New feature"},
	}})
	m = updated.(Model)

	if len(m.prs) != 1 {
		t.Error("new PR should be added")
	}
	if m.prBadge != 1 {
		t.Error("PR badge should increment when on different tab")
	}
}

func TestModel_EventReceived_NoBadgeOnActiveTab(t *testing.T) {
	m := New(nil)
	m.activeTab = tabIssues

	updated, _ := m.Update(eventReceivedMsg{event: event.Event{
		Type: event.TypeNewIssue,
		Item: &provider.Item{ID: "o/r/issues/99", Number: 99, Title: "New bug"},
	}})
	m = updated.(Model)

	if m.issueBadge != 0 {
		t.Error("badge should not increment when on the active tab")
	}
}

func TestModel_BadgeClearsOnTabSwitch(t *testing.T) {
	m := New(nil)
	m.activeTab = tabIssues
	m.prBadge = 3

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	if m.prBadge != 0 {
		t.Error("badge should clear when switching to that tab")
	}
}

func TestModel_Quit(t *testing.T) {
	m := New(nil)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil {
		t.Error("q should produce a quit command")
	}
}

func TestModel_WindowResize(t *testing.T) {
	m := New(nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	if m.width != 120 || m.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

func TestModel_ErrorMessage(t *testing.T) {
	m := New(nil)
	updated, _ := m.Update(errMsg{err: fmt.Errorf("connection failed")})
	m = updated.(Model)

	if m.err == nil {
		t.Error("error should be set")
	}
	view := m.View()
	if !containsString(view.Content, "connection failed") {
		t.Error("error should appear in view")
	}
}

func TestModel_View_ShowsItems(t *testing.T) {
	m := New(nil)
	m.width = 80
	issues, _ := testItems()
	m.issues = issues

	view := m.View()
	if !containsString(view.Content, "Bug report") {
		t.Error("view should show issue title")
	}
}

func TestModel_SelectedItem(t *testing.T) {
	m := New(nil)
	issues, _ := testItems()
	m.issues = issues
	m.cursor = 0

	item := m.selectedItem()
	if item == nil || item.Number != 1 {
		t.Error("should select first issue")
	}
}

func TestModel_SelectedItem_Empty(t *testing.T) {
	m := New(nil)
	item := m.selectedItem()
	if item != nil {
		t.Error("should return nil for empty list")
	}
}

func TestModel_UpdatedEvent(t *testing.T) {
	m := New(nil)
	m.issues = []provider.Item{
		{ID: "o/r/issues/1", Number: 1, Title: "Old title", UpdatedAt: time.Now()},
	}

	updated, _ := m.Update(eventReceivedMsg{event: event.Event{
		Type: event.TypeIssueUpdated,
		Item: &provider.Item{ID: "o/r/issues/1", Number: 1, Title: "New title"},
	}})
	m = updated.(Model)

	if len(m.issues) != 1 {
		t.Error("should still have 1 issue")
	}
	if m.issues[0].Title != "New title" {
		t.Errorf("expected updated title, got %q", m.issues[0].Title)
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

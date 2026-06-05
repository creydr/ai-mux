package attach

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/provider"
)

func testItem() *provider.Item {
	return &provider.Item{
		ID:     "o/r/issues/1",
		Number: 1,
		Title:  "Test issue",
		Body:   "This is a test issue body",
		Author: "user1",
		State:  "open",
		Type:   provider.ItemTypeIssue,
		Repo:   provider.RepoRef{Owner: "o", Repo: "r"},
		URL:    "https://github.com/o/r/issues/1",
	}
}

func TestModel_InitWithoutConn(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil without a connection")
	}
}

func TestModel_ItemLoaded(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)

	updated, _ := m.Update(itemLoadedMsg{item: testItem()})
	m = updated.(Model)

	if m.item == nil {
		t.Fatal("item should be loaded")
	}
	if m.item.Title != "Test issue" {
		t.Errorf("expected title 'Test issue', got %q", m.item.Title)
	}
}

func TestModel_ReviewsLoaded(t *testing.T) {
	ref := Ref{Type: provider.ItemTypePR, Owner: "o", Repo: "r", Number: 10}
	m := New(nil, ref)

	reviews := []provider.Review{
		{Author: "reviewer", State: "APPROVED", Body: "LGTM"},
	}
	updated, _ := m.Update(reviewsLoadedMsg{reviews: reviews})
	m = updated.(Model)

	if len(m.reviews) != 1 {
		t.Fatalf("expected 1 review, got %d", len(m.reviews))
	}
}

func TestModel_CommentsLoaded(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)

	comments := []provider.Comment{
		{Author: "user2", Body: "I can reproduce this"},
	}
	updated, _ := m.Update(commentsLoadedMsg{comments: comments})
	m = updated.(Model)

	if len(m.comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(m.comments))
	}
}

func TestModel_ScrollDown(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)
	m.item = testItem()

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = updated.(Model)

	if m.scroll != 1 {
		t.Errorf("expected scroll 1, got %d", m.scroll)
	}
}

func TestModel_ScrollUp(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)
	m.scroll = 3

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'k'})
	m = updated.(Model)

	if m.scroll != 2 {
		t.Errorf("expected scroll 2, got %d", m.scroll)
	}
}

func TestModel_ScrollUp_Floor(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)
	m.scroll = 0

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'k'})
	m = updated.(Model)

	if m.scroll != 0 {
		t.Error("scroll should not go below 0")
	}
}

func TestModel_Quit(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil {
		t.Error("q should produce a quit command")
	}
}

func TestModel_WindowResize(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = updated.(Model)

	if m.width != 100 || m.height != 50 {
		t.Errorf("expected 100x50, got %dx%d", m.width, m.height)
	}
}

func TestModel_View_ShowsItem(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)
	m.item = testItem()

	view := m.View()
	if !contains(view.Content, "Test issue") {
		t.Error("view should contain item title")
	}
	if !contains(view.Content, "user1") {
		t.Error("view should contain author")
	}
}

func TestModel_View_ShowsReviews(t *testing.T) {
	ref := Ref{Type: provider.ItemTypePR, Owner: "o", Repo: "r", Number: 10}
	m := New(nil, ref)
	m.item = &provider.Item{Number: 10, Title: "PR", Type: provider.ItemTypePR, Repo: provider.RepoRef{Owner: "o", Repo: "r"}}
	m.reviews = []provider.Review{
		{Author: "reviewer", State: "APPROVED", Body: "looks good"},
	}

	view := m.View()
	if !contains(view.Content, "Reviews") {
		t.Error("view should show reviews section")
	}
}

func TestModel_View_ShowsComments(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)
	m.item = testItem()
	m.comments = []provider.Comment{
		{Author: "commenter", Body: "interesting"},
	}

	view := m.View()
	if !contains(view.Content, "Comments") {
		t.Error("view should show comments section")
	}
}

func TestModel_Error(t *testing.T) {
	ref := Ref{Type: provider.ItemTypeIssue, Owner: "o", Repo: "r", Number: 1}
	m := New(nil, ref)

	updated, _ := m.Update(errMsg{err: fmt.Errorf("oops")})
	m = updated.(Model)

	view := m.View()
	if !contains(view.Content, "oops") {
		t.Error("view should show error")
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

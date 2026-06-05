package assign

import (
	"context"
	"fmt"
	"testing"

	"github.com/creydr/ai-mux/internal/action"
	"github.com/creydr/ai-mux/internal/provider"
)

type mockAssigner struct {
	assigned []assignCall
	err      error
}

type assignCall struct {
	repo     provider.RepoRef
	number   int
	username string
}

func (m *mockAssigner) AssignUser(_ context.Context, repo provider.RepoRef, number int, username string) error {
	if m.err != nil {
		return m.err
	}
	m.assigned = append(m.assigned, assignCall{repo, number, username})
	return nil
}

func TestAssign_Execute(t *testing.T) {
	assigner := &mockAssigner{}
	a := New(assigner, "testuser")

	result := a.Execute(context.Background(), action.Context{
		Item: &provider.Item{
			Number: 42,
			Repo:   provider.RepoRef{Owner: "o", Repo: "r"},
		},
	})

	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if len(assigner.assigned) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assigner.assigned))
	}
	if assigner.assigned[0].username != "testuser" {
		t.Errorf("expected testuser, got %q", assigner.assigned[0].username)
	}
	if assigner.assigned[0].number != 42 {
		t.Errorf("expected number 42, got %d", assigner.assigned[0].number)
	}
}

func TestAssign_Execute_Error(t *testing.T) {
	assigner := &mockAssigner{err: fmt.Errorf("permission denied")}
	a := New(assigner, "testuser")

	result := a.Execute(context.Background(), action.Context{
		Item: &provider.Item{Number: 1, Repo: provider.RepoRef{Owner: "o", Repo: "r"}},
	})

	if result.Success {
		t.Error("expected failure")
	}
	if result.Error == nil {
		t.Error("expected error")
	}
}

func TestAssign_Execute_NilItem(t *testing.T) {
	assigner := &mockAssigner{}
	a := New(assigner, "testuser")

	result := a.Execute(context.Background(), action.Context{})
	if result.Success {
		t.Error("expected failure for nil item")
	}
}

func TestAssign_Type(t *testing.T) {
	a := New(&mockAssigner{}, "user")
	if a.Type() != action.ActionAssignSelf {
		t.Errorf("expected assign_self, got %s", a.Type())
	}
}

func TestAssign_Name(t *testing.T) {
	a := New(&mockAssigner{}, "user")
	if a.Name() != "Assign to me" {
		t.Errorf("expected 'Assign to me', got %q", a.Name())
	}
}

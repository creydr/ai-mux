package action

import (
	"context"
	"testing"
)

type stubAction struct {
	actionType ActionType
	name       string
}

func (s *stubAction) Type() ActionType                            { return s.actionType }
func (s *stubAction) Name() string                                { return s.name }
func (s *stubAction) Execute(_ context.Context, _ Context) Result { return Result{Success: true} }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	a := &stubAction{actionType: ActionOpenBrowser, name: "Browser"}

	r.Register(a)

	got, ok := r.Get(ActionOpenBrowser)
	if !ok {
		t.Fatal("expected action to be found")
	}
	if got.Name() != "Browser" {
		t.Errorf("expected 'Browser', got %q", got.Name())
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get(ActionFixIssue)
	if ok {
		t.Error("expected action to not be found")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubAction{actionType: ActionOpenBrowser, name: "Browser"})
	r.Register(&stubAction{actionType: ActionFixIssue, name: "Fix"})

	list := r.List()
	if len(list) != 2 {
		t.Errorf("expected 2 actions, got %d", len(list))
	}
}

func TestRegistry_Register_Replaces(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubAction{actionType: ActionOpenBrowser, name: "Old"})
	r.Register(&stubAction{actionType: ActionOpenBrowser, name: "New"})

	got, _ := r.Get(ActionOpenBrowser)
	if got.Name() != "New" {
		t.Errorf("expected 'New', got %q", got.Name())
	}
}

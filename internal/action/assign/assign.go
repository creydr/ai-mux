package assign

import (
	"context"
	"fmt"

	"github.com/creydr/ai-mux/internal/action"
	"github.com/creydr/ai-mux/internal/provider"
)

type Action struct {
	assigner provider.Assigner
	username string
}

func New(assigner provider.Assigner, username string) *Action {
	return &Action{
		assigner: assigner,
		username: username,
	}
}

func (a *Action) Type() action.ActionType {
	return action.ActionAssignSelf
}

func (a *Action) Name() string {
	return "Assign to me"
}

func (a *Action) Execute(ctx context.Context, actCtx action.Context) action.Result {
	if actCtx.Item == nil {
		return action.Result{Error: fmt.Errorf("no item provided")}
	}

	if err := a.assigner.AssignUser(ctx, actCtx.Item.Repo, actCtx.Item.Number, a.username); err != nil {
		return action.Result{Error: fmt.Errorf("assigning: %w", err)}
	}

	return action.Result{
		Success: true,
		Message: fmt.Sprintf("Assigned %s to #%d", a.username, actCtx.Item.Number),
	}
}

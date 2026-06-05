package action

import (
	"context"

	"github.com/creydr/ai-mux/internal/config"
	"github.com/creydr/ai-mux/internal/provider"
)

type ActionType string

const (
	ActionFixIssue    ActionType = "fix_issue"
	ActionReviewPR    ActionType = "review_pr"
	ActionOpenBrowser ActionType = "open_browser"
	ActionAssignSelf  ActionType = "assign_self"
)

type Context struct {
	Item   *provider.Item
	Repo   config.RepoConfig
	Config *config.Config
}

type Result struct {
	Success bool
	Message string
	Error   error
}

type Action interface {
	Name() string
	Type() ActionType
	Execute(ctx context.Context, actCtx Context) Result
}

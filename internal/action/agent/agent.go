package agent

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/creydr/ai-mux/internal/action"
	"github.com/creydr/ai-mux/internal/worktree"
)

type AgentAction struct {
	actionType action.ActionType
	runner     *Runner
	worktrees  *worktree.Manager
	agentName  string
}

func NewFixIssueAction(runner *Runner, worktrees *worktree.Manager, agentName string) *AgentAction {
	return &AgentAction{
		actionType: action.ActionFixIssue,
		runner:     runner,
		worktrees:  worktrees,
		agentName:  agentName,
	}
}

func NewReviewPRAction(runner *Runner, worktrees *worktree.Manager, agentName string) *AgentAction {
	return &AgentAction{
		actionType: action.ActionReviewPR,
		runner:     runner,
		worktrees:  worktrees,
		agentName:  agentName,
	}
}

func (a *AgentAction) Type() action.ActionType {
	return a.actionType
}

func (a *AgentAction) Name() string {
	switch a.actionType {
	case action.ActionFixIssue:
		return fmt.Sprintf("Fix with %s", a.agentName)
	case action.ActionReviewPR:
		return fmt.Sprintf("Review with %s", a.agentName)
	default:
		return fmt.Sprintf("Run %s", a.agentName)
	}
}

func (a *AgentAction) Execute(_ context.Context, actCtx action.Context) action.Result {
	if actCtx.Item == nil {
		return action.Result{Error: fmt.Errorf("no item provided")}
	}

	wtName := fmt.Sprintf("%s-%s-%d", a.actionType, a.agentName, actCtx.Item.Number)
	wtPath, err := a.worktrees.Create(actCtx.Repo.Path, wtName)
	if err != nil {
		return action.Result{Error: fmt.Errorf("creating worktree: %w", err)}
	}

	data := TemplateData{
		Item:     actCtx.Item,
		Repo:     actCtx.Repo.Name,
		RepoPath: actCtx.Repo.Path,
		Worktree: wtPath,
	}

	cmdStr, err := a.runner.BuildCommand(a.agentName, string(a.actionType), data)
	if err != nil {
		return action.Result{Error: fmt.Errorf("building command: %w", err)}
	}

	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = wtPath
	if err := cmd.Run(); err != nil {
		return action.Result{
			Error:   fmt.Errorf("agent exited with error: %w", err),
			Message: fmt.Sprintf("Agent %s finished with error in %s", a.agentName, wtPath),
		}
	}

	postSession := a.runner.GetPostSession(a.agentName)
	handler := worktree.NewPostSessionHandler(postSession, a.worktrees)
	if err := handler.Handle(actCtx.Repo.Path, wtPath, actCtx.Item.Title); err != nil {
		return action.Result{
			Success: true,
			Error:   fmt.Errorf("post-session %s: %w", postSession, err),
			Message: fmt.Sprintf("Agent completed but post-session failed: %v", err),
		}
	}

	return action.Result{
		Success: true,
		Message: fmt.Sprintf("Agent %s completed successfully", a.agentName),
	}
}

package agent

import (
	"testing"

	"github.com/creydr/ai-mux/internal/config"
	"github.com/creydr/ai-mux/internal/provider"
)

func testRunner() *Runner {
	return NewRunner([]config.AgentConfig{
		{
			Name:        "claude",
			Command:     "claude",
			PostSession: "auto-pr",
			ArgsTemplates: map[string]string{
				"fix_issue": "--prompt \"Fix issue #{{.Item.Number}}: {{.Item.Title}}\"",
				"review_pr": "--prompt \"Review PR #{{.Item.Number}}\"",
			},
		},
		{
			Name:        "gemini",
			Command:     "gemini-cli run",
			PostSession: "keep",
			ArgsTemplates: map[string]string{
				"fix_issue": "fix {{.Item.Number}}",
			},
		},
	})
}

func TestRunner_HasAgent(t *testing.T) {
	r := testRunner()

	if !r.HasAgent("claude") {
		t.Error("should have claude")
	}
	if !r.HasAgent("gemini") {
		t.Error("should have gemini")
	}
	if r.HasAgent("gpt") {
		t.Error("should not have gpt")
	}
}

func TestRunner_BuildCommand(t *testing.T) {
	r := testRunner()
	data := TemplateData{
		Item:     &provider.Item{Number: 42, Title: "Fix bug"},
		Repo:     "owner/repo",
		RepoPath: "/tmp/repo",
		Worktree: "/tmp/repo/.worktrees/fix-42",
	}

	cmd, err := r.BuildCommand("claude", "fix_issue", data)
	if err != nil {
		t.Fatal(err)
	}

	if cmd.Path == "" {
		t.Error("command path should be set")
	}
	if cmd.Dir != "/tmp/repo/.worktrees/fix-42" {
		t.Errorf("expected worktree dir, got %q", cmd.Dir)
	}
}

func TestRunner_BuildCommand_MultiWordCommand(t *testing.T) {
	r := testRunner()
	data := TemplateData{
		Item:     &provider.Item{Number: 1, Title: "test"},
		Worktree: "/tmp/wt",
	}

	cmd, err := r.BuildCommand("gemini", "fix_issue", data)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Args[0] != "gemini-cli" {
		t.Errorf("expected 'gemini-cli', got %q", cmd.Args[0])
	}
}

func TestRunner_BuildCommand_UnknownAgent(t *testing.T) {
	r := testRunner()
	_, err := r.BuildCommand("unknown", "fix_issue", TemplateData{})
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestRunner_BuildCommand_UnknownAction(t *testing.T) {
	r := testRunner()
	_, err := r.BuildCommand("claude", "unknown_action", TemplateData{Item: &provider.Item{}})
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}

func TestRunner_GetPostSession(t *testing.T) {
	r := testRunner()

	if ps := r.GetPostSession("claude"); ps != "auto-pr" {
		t.Errorf("expected auto-pr, got %q", ps)
	}
	if ps := r.GetPostSession("gemini"); ps != "keep" {
		t.Errorf("expected keep, got %q", ps)
	}
	if ps := r.GetPostSession("unknown"); ps != "keep" {
		t.Errorf("expected keep for unknown, got %q", ps)
	}
}

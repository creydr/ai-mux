package agent

import (
	"testing"

	"github.com/creydr/ai-mux/internal/config"
)

func testRunner() *Runner {
	return NewRunner([]config.AgentConfig{
		{
			Name:    "claude",
			Command: "claude",
			Args:    []string{"--dangerously-skip-permissions"},
		},
		{
			Name:    "gemini",
			Command: "gemini-cli run",
		},
		{
			Name:    "plain",
			Command: "plain-agent",
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

func TestRunner_GetCommand(t *testing.T) {
	r := testRunner()

	if cmd := r.GetCommand("claude"); cmd != "claude --dangerously-skip-permissions" {
		t.Errorf("expected claude with args, got %q", cmd)
	}
	if cmd := r.GetCommand("gemini"); cmd != "gemini-cli run" {
		t.Errorf("expected gemini-cli run, got %q", cmd)
	}
	if cmd := r.GetCommand("plain"); cmd != "plain-agent" {
		t.Errorf("expected plain-agent, got %q", cmd)
	}
	if cmd := r.GetCommand("unknown"); cmd != "unknown" {
		t.Errorf("expected unknown as fallback, got %q", cmd)
	}
}

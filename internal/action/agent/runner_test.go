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
		},
		{
			Name:    "gemini",
			Command: "gemini-cli run",
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

	if cmd := r.GetCommand("claude"); cmd != "claude" {
		t.Errorf("expected claude, got %q", cmd)
	}
	if cmd := r.GetCommand("gemini"); cmd != "gemini-cli run" {
		t.Errorf("expected gemini-cli run, got %q", cmd)
	}
	if cmd := r.GetCommand("unknown"); cmd != "unknown" {
		t.Errorf("expected unknown as fallback, got %q", cmd)
	}
}

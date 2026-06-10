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

	if cmd := r.GetCommand("claude"); cmd != "'claude' '--dangerously-skip-permissions'" {
		t.Errorf("expected quoted claude with args, got %q", cmd)
	}
	if cmd := r.GetCommand("gemini"); cmd != "'gemini-cli run'" {
		t.Errorf("expected quoted gemini-cli run, got %q", cmd)
	}
	if cmd := r.GetCommand("plain"); cmd != "'plain-agent'" {
		t.Errorf("expected quoted plain-agent, got %q", cmd)
	}
	if cmd := r.GetCommand("unknown"); cmd != "'unknown'" {
		t.Errorf("expected quoted unknown as fallback, got %q", cmd)
	}
}

func TestRunner_GetCommand_ShellEscaping(t *testing.T) {
	r := NewRunner([]config.AgentConfig{
		{Name: "spaces", Command: "my agent", Args: []string{"--title", "hello world"}},
		{Name: "quotes", Command: "agent", Args: []string{"it's", "a test"}},
		{Name: "meta", Command: "agent", Args: []string{"; rm -rf /", "$(whoami)", "|cat /etc/passwd"}},
	})

	if cmd := r.GetCommand("spaces"); cmd != "'my agent' '--title' 'hello world'" {
		t.Errorf("spaces not escaped correctly, got %q", cmd)
	}
	if cmd := r.GetCommand("quotes"); cmd != "'agent' 'it'\\''s' 'a test'" {
		t.Errorf("quotes not escaped correctly, got %q", cmd)
	}
	if cmd := r.GetCommand("meta"); cmd != "'agent' '; rm -rf /' '$(whoami)' '|cat /etc/passwd'" {
		t.Errorf("metacharacters not escaped correctly, got %q", cmd)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"it's", "'it'\\''s'"},
		{"$(cmd)", "'$(cmd)'"},
		{"; rm -rf /", "'; rm -rf /'"},
		{"", "''"},
	}
	for _, tt := range tests {
		if got := shellQuote(tt.input); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

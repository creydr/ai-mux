package agent

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"text/template"

	"github.com/creydr/ai-mux/internal/config"
	"github.com/creydr/ai-mux/internal/provider"
)

type TemplateData struct {
	Item     *provider.Item
	Repo     string
	RepoPath string
	Worktree string
}

type Runner struct {
	agents map[string]config.AgentConfig
}

func NewRunner(agents []config.AgentConfig) *Runner {
	m := make(map[string]config.AgentConfig, len(agents))
	for _, a := range agents {
		m[a.Name] = a
	}
	return &Runner{agents: m}
}

func (r *Runner) HasAgent(name string) bool {
	_, ok := r.agents[name]
	return ok
}

func (r *Runner) BuildCommand(agentName, actionType string, data TemplateData) (*exec.Cmd, error) {
	agent, ok := r.agents[agentName]
	if !ok {
		return nil, fmt.Errorf("agent %q not configured", agentName)
	}

	tmplStr, ok := agent.ArgsTemplates[actionType]
	if !ok {
		return nil, fmt.Errorf("agent %q has no template for action %q", agentName, actionType)
	}

	tmpl, err := template.New("args").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parsing args template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing args template: %w", err)
	}

	args := strings.Fields(buf.String())
	parts := strings.Fields(agent.Command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("agent %q has empty command", agentName)
	}

	cmd := exec.Command(parts[0], append(parts[1:], args...)...)
	cmd.Dir = data.Worktree
	return cmd, nil
}

func (r *Runner) GetPostSession(agentName string) string {
	if a, ok := r.agents[agentName]; ok {
		return a.PostSession
	}
	return "keep"
}

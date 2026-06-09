package agent

import (
	"strings"

	"github.com/creydr/ai-mux/internal/config"
)

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

func (r *Runner) GetCommand(agentName string) string {
	if a, ok := r.agents[agentName]; ok {
		if len(a.Args) == 0 {
			return a.Command
		}
		return a.Command + " " + strings.Join(a.Args, " ")
	}
	return agentName
}

package agent

import (
	"strings"

	"github.com/creydr/ai-mux/internal/config"
)

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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

func (r *Runner) GetCommand(agentName string) string {
	if a, ok := r.agents[agentName]; ok {
		if len(a.Args) == 0 {
			return shellQuote(a.Command)
		}
		quoted := make([]string, len(a.Args))
		for i, arg := range a.Args {
			quoted[i] = shellQuote(arg)
		}
		return shellQuote(a.Command) + " " + strings.Join(quoted, " ")
	}
	return shellQuote(agentName)
}

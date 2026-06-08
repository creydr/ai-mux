package commands

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
	"github.com/creydr/ai-mux/internal/tui/dashboard"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the monitoring dashboard",
	RunE:  runDashboard,
}

func runDashboard(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	transport := jsonlines.NewTransport()
	conn, err := transport.Dial(cfg.ACP.Socket)
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w (is the daemon running?)", err)
	}
	defer conn.Close()

	var agentNames []string
	for _, a := range cfg.Agents {
		agentNames = append(agentNames, a.Name)
	}
	m := dashboard.New(conn, cfg.Dashboard.ItemsPerRepo, agentNames, cfg.DefaultAgent)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running dashboard: %w", err)
	}
	return nil
}

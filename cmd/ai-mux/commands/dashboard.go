package commands

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/protocol"
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
	conn, err := transport.Dial(cfg.Daemon.Socket)
	if err != nil && !isDaemonRunning() {
		pid, startErr := startDaemonBackground()
		if startErr != nil {
			return fmt.Errorf("daemon not running and failed to auto-start: %w", startErr)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "daemon auto-started (pid %d), connecting...\n", pid)

		conn, err = waitForDaemon(transport, cfg.Daemon.Socket, 5*time.Second)
	}
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()

	var agentNames []string
	for _, a := range cfg.Agents {
		agentNames = append(agentNames, a.Name)
	}
	var repoNames []string
	for _, r := range cfg.Repos {
		repoNames = append(repoNames, r.Name)
	}
	m := dashboard.New(conn, cfg.Dashboard.ItemsPerRepo, agentNames, cfg.Jira != nil, repoNames)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running dashboard: %w", err)
	}
	return nil
}

func waitForDaemon(transport protocol.Transport, socket string, timeout time.Duration) (protocol.Conn, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := transport.Dial(socket)
		if err == nil {
			return conn, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("daemon did not become ready within %s", timeout)
}

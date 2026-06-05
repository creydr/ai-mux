package commands

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
	"github.com/creydr/ai-mux/internal/tui/attach"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach <type/owner/repo/number>",
	Short: "Attach to a specific issue or PR",
	Long:  "Open a focused view for a single issue or PR. Example: ai-mux attach pr/owner/repo/123",
	Args:  cobra.ExactArgs(1),
	RunE:  runAttach,
}

func runAttach(cmd *cobra.Command, args []string) error {
	ref, err := attach.ParseRef(args[0])
	if err != nil {
		return err
	}

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

	m := attach.New(conn, ref)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running attach view: %w", err)
	}
	return nil
}

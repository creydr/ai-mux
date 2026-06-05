package commands

import (
	"os"

	"github.com/creydr/ai-mux/internal/acp"
	"github.com/spf13/cobra"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Start ACP agent for IDE integration (JSON-RPC over stdio)",
	RunE:  runACP,
}

func init() {
	rootCmd.AddCommand(acpCmd)
}

func runACP(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	agent := acp.NewAgent(os.Stdin, os.Stdout, cfg.ACP.Socket)
	return agent.Serve()
}

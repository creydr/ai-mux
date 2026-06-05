package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach <type/owner/repo/number>",
	Short: "Attach to a specific issue or PR",
	Long:  "Open a focused view for a single issue or PR. Example: ai-mux attach pr/owner/repo/123",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not yet implemented")
	},
}

package commands

import (
	"fmt"

	"github.com/creydr/ai-mux/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgPath string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "ai-mux",
	Short: "Monitor GitHub repositories from your terminal",
	Long:  "ai-mux watches multiple GitHub repositories for new issues, PRs, and review activity, with actionable integrations for AI agents and IDE support.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file (default ~/.config/ai-mux/config.yaml)")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(attachCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func loadConfig() (*config.Config, error) {
	path := cfgPath
	if path == "" {
		path = config.DefaultPath()
	}
	c, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return c, nil
}

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/creydr/ai-mux/internal/daemon"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
	"github.com/creydr/ai-mux/internal/provider/github"
	"github.com/creydr/ai-mux/internal/store/jsonfile"
	"github.com/spf13/cobra"
)

var foreground bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the ai-mux daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon",
	RunE:  runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE:  runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE:  runDaemonStatus,
}

func init() {
	daemonStartCmd.Flags().BoolVar(&foreground, "foreground", false, "run in foreground (don't detach)")
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
}

func pidFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "ai-mux", "daemon.pid")
}

func stateFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "ai-mux", "state.json")
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	pidPath := pidFilePath()
	if pid, err := daemon.ReadPIDFile(pidPath); err == nil && daemon.IsRunning(pid) {
		return fmt.Errorf("daemon already running (pid %d)", pid)
	}

	prov, err := github.NewFromGHCLI()
	if err != nil {
		return fmt.Errorf("setting up GitHub provider: %w", err)
	}

	st, err := jsonfile.New(stateFilePath())
	if err != nil {
		return fmt.Errorf("opening state store: %w", err)
	}

	transport := jsonlines.NewTransport()
	d, err := daemon.New(cfg, prov, st, transport)
	if err != nil {
		return err
	}

	if err := daemon.WritePIDFile(pidPath); err != nil {
		return fmt.Errorf("writing pid file: %w", err)
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(cmd.OutOrStdout(), "shutting down...")
		cancel()
	}()

	fmt.Fprintf(cmd.OutOrStdout(), "daemon started (pid %d), listening on %s\n", os.Getpid(), cfg.Daemon.Socket)
	err = d.Start(ctx)
	daemon.RemovePIDFile(pidPath)
	return err
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	pidPath := pidFilePath()
	pid, err := daemon.ReadPIDFile(pidPath)
	if err != nil {
		return fmt.Errorf("daemon not running (no pid file)")
	}
	if !daemon.IsRunning(pid) {
		daemon.RemovePIDFile(pidPath)
		return fmt.Errorf("daemon not running (stale pid file)")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process: %w", err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("sending signal: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "sent stop signal to daemon (pid %d)\n", pid)
	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	pidPath := pidFilePath()
	pid, err := daemon.ReadPIDFile(pidPath)
	if err != nil || !daemon.IsRunning(pid) {
		fmt.Fprintln(cmd.OutOrStdout(), "daemon is not running")
		return nil
	}

	transport := jsonlines.NewTransport()
	conn, err := transport.Dial(cfg.Daemon.Socket)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "daemon is running (pid %d) but not reachable\n", pid)
		return nil
	}
	defer conn.Close()

	msg, err := protocol.NewRequest(protocol.MsgGetStatus, "status-1", nil)
	if err != nil {
		return err
	}
	if err := conn.Send(msg); err != nil {
		return err
	}

	resp, err := conn.Receive()
	if err != nil {
		return err
	}

	var status protocol.StatusPayload
	if err := json.Unmarshal(resp.Payload, &status); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "daemon running (pid %d)\n", pid)
	fmt.Fprintf(cmd.OutOrStdout(), "  uptime:  %s\n", status.Uptime)
	fmt.Fprintf(cmd.OutOrStdout(), "  repos:   %v\n", status.Repos)
	fmt.Fprintf(cmd.OutOrStdout(), "  clients: %d\n", status.Clients)
	return nil
}

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"text/template"

	"github.com/creydr/ai-mux/internal/daemon"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
	"github.com/creydr/ai-mux/internal/provider/github"
	"github.com/creydr/ai-mux/internal/store/jsonfile"
	"github.com/spf13/cobra"
)

var background bool

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

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the daemon as a system service",
	RunE:  runDaemonInstall,
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the daemon system service",
	RunE:  runDaemonUninstall,
}

func init() {
	daemonStartCmd.Flags().BoolVar(&background, "background", false, "detach and run in the background")
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonInstallCmd)
	daemonCmd.AddCommand(daemonUninstallCmd)
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
	if background && os.Getenv("AI_MUX_DAEMON") == "" {
		return startInBackground(cmd)
	}

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

func startInBackground(cmd *cobra.Command) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}

	childArgs := []string{"daemon", "start"}
	if cfgPath != "" {
		childArgs = append(childArgs, "--config", cfgPath)
	}

	child := exec.Command(exe, childArgs...)
	child.Env = append(os.Environ(), "AI_MUX_DAEMON=1")
	child.Stdout = nil
	child.Stderr = nil
	child.Stdin = nil

	if err := child.Start(); err != nil {
		return fmt.Errorf("starting background daemon: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "daemon started in background (pid %d)\n", child.Process.Pid)
	return nil
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

type serviceParams struct {
	ExePath    string
	ConfigPath string
	LogPath    string
}

var systemdTemplate = template.Must(template.New("systemd").Parse(`[Unit]
Description=ai-mux daemon
After=network.target

[Service]
Type=simple
ExecStart={{.ExePath}} daemon start{{if .ConfigPath}} --config {{.ConfigPath}}{{end}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`))

var launchdTemplate = template.Must(template.New("launchd").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.ai-mux.daemon</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.ExePath}}</string>
		<string>daemon</string>
		<string>start</string>{{if .ConfigPath}}
		<string>--config</string>
		<string>{{.ConfigPath}}</string>{{end}}
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.LogPath}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogPath}}</string>
</dict>
</plist>
`))

func serviceFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	switch runtime.GOOS {
	case "linux":
		return filepath.Join(home, ".config", "systemd", "user", "ai-mux.service"), nil
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", "com.ai-mux.daemon.plist"), nil
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func runDaemonInstall(cmd *cobra.Command, args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}

	cfgArg := cfgPath

	svcPath, err := serviceFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(svcPath), 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.Create(svcPath)
	if err != nil {
		return fmt.Errorf("creating service file: %w", err)
	}
	defer f.Close()

	params := serviceParams{
		ExePath:    exe,
		ConfigPath: cfgArg,
	}

	switch runtime.GOOS {
	case "linux":
		if err := systemdTemplate.Execute(f, params); err != nil {
			return fmt.Errorf("writing service file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote systemd unit to %s\n\n", svcPath)
		fmt.Fprintln(cmd.OutOrStdout(), "to enable and start:")
		fmt.Fprintln(cmd.OutOrStdout(), "  systemctl --user daemon-reload")
		fmt.Fprintln(cmd.OutOrStdout(), "  systemctl --user enable --now ai-mux")
	case "darwin":
		home, _ := os.UserHomeDir()
		params.LogPath = filepath.Join(home, "Library", "Logs", "ai-mux.log")
		if err := launchdTemplate.Execute(f, params); err != nil {
			return fmt.Errorf("writing plist: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "wrote launchd plist to %s\n\n", svcPath)
		fmt.Fprintln(cmd.OutOrStdout(), "to load:")
		fmt.Fprintf(cmd.OutOrStdout(), "  launchctl load %s\n", svcPath)
	}

	return nil
}

func runDaemonUninstall(cmd *cobra.Command, args []string) error {
	svcPath, err := serviceFilePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(svcPath); os.IsNotExist(err) {
		return fmt.Errorf("service file not found at %s", svcPath)
	}

	if err := os.Remove(svcPath); err != nil {
		return fmt.Errorf("removing service file: %w", err)
	}

	switch runtime.GOOS {
	case "linux":
		fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n\n", svcPath)
		fmt.Fprintln(cmd.OutOrStdout(), "to stop and disable:")
		fmt.Fprintln(cmd.OutOrStdout(), "  systemctl --user disable --now ai-mux")
		fmt.Fprintln(cmd.OutOrStdout(), "  systemctl --user daemon-reload")
	case "darwin":
		fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n\n", svcPath)
		fmt.Fprintln(cmd.OutOrStdout(), "to unload (if currently running):")
		fmt.Fprintf(cmd.OutOrStdout(), "  launchctl unload %s\n", svcPath)
	}

	return nil
}

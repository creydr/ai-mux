package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
	"github.com/spf13/cobra"
)

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage agent sessions",
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active sessions",
	RunE:  runSessionList,
}

var sessionAttachCmd = &cobra.Command{
	Use:   "attach <session-id>",
	Short: "Attach to a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionAttach,
}

var sessionRenameCmd = &cobra.Command{
	Use:   "rename <session-id> <name>",
	Short: "Rename a session",
	Args:  cobra.ExactArgs(2),
	RunE:  runSessionRename,
}

func init() {
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionAttachCmd)
	sessionCmd.AddCommand(sessionRenameCmd)
}

func runSessionList(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	transport := jsonlines.NewTransport()
	conn, err := transport.Dial(cfg.Daemon.Socket)
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w (is the daemon running?)", err)
	}
	defer conn.Close()

	resp, err := protocol.SendRequest(conn, protocol.MsgSessionList, "cli-sessions", nil, protocol.DefaultTimeout)
	if err != nil {
		return err
	}
	if resp.Type == protocol.MsgError {
		return fmt.Errorf("daemon error")
	}

	var payload protocol.SessionListPayload
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	if len(payload.Sessions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no active sessions")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tREPO\tNUMBER\tAGENT\tSTATUS\tCREATED")
	for _, s := range payload.Sessions {
		name := s.Name
		if name == "" {
			name = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n", s.ID, name, s.Repo, s.Number, s.Agent, s.Status, s.CreatedAt)
	}
	w.Flush()
	return nil
}

func runSessionAttach(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	transport := jsonlines.NewTransport()
	conn, err := transport.Dial(cfg.Daemon.Socket)
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w (is the daemon running?)", err)
	}
	defer conn.Close()

	resp, err := protocol.SendRequest(conn, protocol.MsgSessionList, "cli-find", nil, protocol.DefaultTimeout)
	if err != nil {
		return err
	}

	var payload protocol.SessionListPayload
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	var found *protocol.SessionPayload
	for _, s := range payload.Sessions {
		if s.ID == sessionID {
			found = &s
			break
		}
	}
	if found == nil {
		return fmt.Errorf("session %q not found", sessionID)
	}

	switch found.Status {
	case "running", "pending":
		return tmuxAttach(sessionID, found.Name)
	default:
		return streamOutput(conn, sessionID)
	}
}

func tmuxAttach(sessionID, sessionName string) error {
	tmuxName := "ai-mux-" + sessionID
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}
	exe, _ := os.Executable()
	_ = exec.Command(tmuxPath, "set-option", "-t", tmuxName, "status-right",
		" ctrl-b d: detach | ctrl-b n: rename ").Run()
	renameTemplate := fmt.Sprintf(
		`run-shell '%s session rename %s "%%%%" >/dev/null 2>&1 && tmux display-message "Session renamed" || tmux display-message "Rename failed"'`,
		strings.ReplaceAll(exe, "'", "'\\''"),
		strings.ReplaceAll(sessionID, "'", "'\\''"))
	_ = exec.Command(tmuxPath, "bind-key", "-T", "prefix", "n",
		"command-prompt", "-I", sessionName, "-p", "Session name:", renameTemplate).Run()
	return syscall.Exec(tmuxPath, []string{"tmux", "attach-session", "-t", tmuxName}, os.Environ())
}

func runSessionRename(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	name := args[1]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	transport := jsonlines.NewTransport()
	conn, err := transport.Dial(cfg.Daemon.Socket)
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w (is the daemon running?)", err)
	}
	defer conn.Close()

	resp, err := protocol.SendRequest(conn, protocol.MsgSessionRename, "cli-rename", protocol.SessionRenamePayload{
		SessionID: sessionID,
		Name:      name,
	}, protocol.DefaultTimeout)
	if err != nil {
		return err
	}
	if resp.Type == protocol.MsgError {
		return fmt.Errorf("rename failed: %s", protocol.ParseErrorPayload(resp))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "session %s renamed to %q\n", sessionID, name)
	return nil
}

func streamOutput(conn protocol.Conn, sessionID string) error {
	resp, err := protocol.SendRequest(conn, protocol.MsgSessionAttach, "cli-attach", protocol.SessionIDPayload{
		SessionID: sessionID,
	}, protocol.DefaultTimeout)
	if err != nil {
		return err
	}
	if resp.Type == protocol.MsgError {
		return fmt.Errorf("attach failed")
	}

	for {
		msg, err := conn.Receive()
		if err != nil {
			return nil
		}
		if msg.Type != protocol.MsgSessionOutput {
			continue
		}
		var out protocol.SessionOutputPayload
		if err := json.Unmarshal(msg.Payload, &out); err != nil {
			continue
		}
		fmt.Print(out.Data)
	}
}

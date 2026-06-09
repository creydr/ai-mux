package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"text/tabwriter"

	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
	"github.com/spf13/cobra"
)

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

func init() {
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionAttachCmd)
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

	req, _ := protocol.NewRequest(protocol.MsgSessionList, "cli-sessions", nil)
	if err := conn.Send(req); err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	resp, err := conn.Receive()
	if err != nil {
		return fmt.Errorf("receiving response: %w", err)
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
	fmt.Fprintln(w, "ID\tREPO\tNUMBER\tAGENT\tSTATUS\tCREATED")
	for _, s := range payload.Sessions {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n", s.ID, s.Repo, s.Number, s.Agent, s.Status, s.CreatedAt)
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

	req, _ := protocol.NewRequest(protocol.MsgSessionList, "cli-find", nil)
	if err := conn.Send(req); err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	resp, err := conn.Receive()
	if err != nil {
		return fmt.Errorf("receiving response: %w", err)
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
		return tmuxAttach(sessionID)
	default:
		return streamOutput(conn, sessionID)
	}
}

func tmuxAttach(sessionID string) error {
	tmuxName := "ai-mux-" + sessionID
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}
	return syscall.Exec(tmuxPath, []string{"tmux", "attach-session", "-t", tmuxName}, os.Environ())
}

func streamOutput(conn protocol.Conn, sessionID string) error {
	req, _ := protocol.NewRequest(protocol.MsgSessionAttach, "cli-attach", protocol.SessionIDPayload{
		SessionID: sessionID,
	})
	if err := conn.Send(req); err != nil {
		return fmt.Errorf("sending attach request: %w", err)
	}

	resp, err := conn.Receive()
	if err != nil {
		return fmt.Errorf("receiving response: %w", err)
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

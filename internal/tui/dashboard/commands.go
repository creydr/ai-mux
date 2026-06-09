package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/action/browser"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

func fetchItemsCmd(conn protocol.Conn, limit int) tea.Cmd {
	return func() tea.Msg {
		issueMsg, _ := protocol.NewRequest(protocol.MsgListIssues, "dash-issues", protocol.ListPayload{Limit: limit})
		if err := conn.Send(issueMsg); err != nil {
			return errMsg{err: err}
		}
		issueResp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		var issues protocol.ItemsPayload
		if err := json.Unmarshal(issueResp.Payload, &issues); err != nil {
			return errMsg{err: fmt.Errorf("parsing issues: %w", err)}
		}
		var issueItems []provider.Item
		if err := json.Unmarshal(issues.Items, &issueItems); err != nil {
			return errMsg{err: fmt.Errorf("parsing issue items: %w", err)}
		}

		prMsg, _ := protocol.NewRequest(protocol.MsgListPRs, "dash-prs", protocol.ListPayload{Limit: limit})
		if err := conn.Send(prMsg); err != nil {
			return errMsg{err: err}
		}
		prResp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		var prs protocol.ItemsPayload
		if err := json.Unmarshal(prResp.Payload, &prs); err != nil {
			return errMsg{err: fmt.Errorf("parsing PRs: %w", err)}
		}
		var prItems []provider.Item
		if err := json.Unmarshal(prs.Items, &prItems); err != nil {
			return errMsg{err: fmt.Errorf("parsing PR items: %w", err)}
		}

		return itemsReceivedMsg{
			issues: issueItems,
			prs:    prItems,
		}
	}
}

func expandRepoCmd(conn protocol.Conn, repo string, itemType provider.ItemType, limit int) tea.Cmd {
	return func() tea.Msg {
		var msgType protocol.MessageType
		if itemType == provider.ItemTypeIssue {
			msgType = protocol.MsgListIssues
		} else {
			msgType = protocol.MsgListPRs
		}

		req, _ := protocol.NewRequest(msgType, "dash-expand", protocol.ListPayload{Repo: repo, Limit: limit})
		if err := conn.Send(req); err != nil {
			return errMsg{err: err}
		}
		resp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		var payload protocol.ItemsPayload
		if err := json.Unmarshal(resp.Payload, &payload); err != nil {
			return errMsg{err: fmt.Errorf("parsing items: %w", err)}
		}
		var items []provider.Item
		if err := json.Unmarshal(payload.Items, &items); err != nil {
			return errMsg{err: fmt.Errorf("parsing items: %w", err)}
		}

		return repoExpandedMsg{
			repo:           repo,
			items:          items,
			itemType:       itemType,
			requestedLimit: limit,
		}
	}
}

func listenEventsCmd(conn protocol.Conn) tea.Cmd {
	return func() tea.Msg {
		msg, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		if msg.Type == protocol.MsgEvent {
			var ev event.Event
			if err := json.Unmarshal(msg.Payload, &ev); err != nil {
				return errMsg{err: fmt.Errorf("parsing event: %w", err)}
			}
			return eventReceivedMsg{event: ev}
		}
		return nil
	}
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		browser.OpenCommand(url).Run()
		return nil
	}
}

func fetchSessionsCmd(conn protocol.Conn) tea.Cmd {
	return func() tea.Msg {
		req, _ := protocol.NewRequest(protocol.MsgSessionList, "dash-sessions", nil)
		if err := conn.Send(req); err != nil {
			return errMsg{err: err}
		}
		resp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		if resp.Type == protocol.MsgError {
			return sessionsReceivedMsg{}
		}
		var payload protocol.SessionListPayload
		if err := json.Unmarshal(resp.Payload, &payload); err != nil {
			return errMsg{err: fmt.Errorf("parsing sessions: %w", err)}
		}
		return sessionsReceivedMsg{sessions: payload.Sessions}
	}
}

func spawnSessionCmd(conn protocol.Conn, repo string, number int, itemType, agent, worktreeAction string) tea.Cmd {
	return func() tea.Msg {
		req, _ := protocol.NewRequest(protocol.MsgSessionSpawn, "dash-spawn", protocol.SessionSpawnPayload{
			Repo:           repo,
			Number:         number,
			ItemType:       itemType,
			Agent:          agent,
			WorktreeAction: worktreeAction,
		})
		if err := conn.Send(req); err != nil {
			return errMsg{err: err}
		}
		resp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		if resp.Type == protocol.MsgWorktreeExists {
			return worktreeExistsMsg{repo: repo, number: number, itemType: itemType, agent: agent}
		}
		if resp.Type == protocol.MsgError {
			var errPayload map[string]string
			if err := json.Unmarshal(resp.Payload, &errPayload); err != nil {
				return errMsg{err: fmt.Errorf("parsing error response: %w", err)}
			}
			return statusMsg{text: "Error: " + errPayload["error"]}
		}
		var sess protocol.SessionPayload
		if err := json.Unmarshal(resp.Payload, &sess); err != nil {
			return errMsg{err: fmt.Errorf("parsing session: %w", err)}
		}
		return sessionSpawnedMsg{session: sess}
	}
}

func stopSessionCmd(conn protocol.Conn, sessionID string) tea.Cmd {
	return func() tea.Msg {
		req, _ := protocol.NewRequest(protocol.MsgSessionStop, "dash-stop", protocol.SessionIDPayload{
			SessionID: sessionID,
		})
		if err := conn.Send(req); err != nil {
			return errMsg{err: err}
		}
		resp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		if resp.Type == protocol.MsgError {
			var errPayload map[string]string
			if err := json.Unmarshal(resp.Payload, &errPayload); err != nil {
				return errMsg{err: fmt.Errorf("parsing error response: %w", err)}
			}
			return statusMsg{text: "Error: " + errPayload["error"]}
		}
		return sessionStoppedMsg{sessionID: sessionID}
	}
}

func attachSessionCmd(conn protocol.Conn, sessionID string) tea.Cmd {
	return func() tea.Msg {
		req, _ := protocol.NewRequest(protocol.MsgSessionAttach, "dash-attach", protocol.SessionIDPayload{
			SessionID: sessionID,
		})
		if err := conn.Send(req); err != nil {
			return errMsg{err: err}
		}
		resp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		if resp.Type == protocol.MsgError {
			var errPayload map[string]string
			if err := json.Unmarshal(resp.Payload, &errPayload); err != nil {
				return errMsg{err: fmt.Errorf("parsing error response: %w", err)}
			}
			return statusMsg{text: "Error: " + errPayload["error"]}
		}
		return sessionAttachedMsg{}
	}
}

func tmuxAttachCmd(sessionID, sessionName string) tea.Cmd {
	tmuxName := "ai-mux-" + sessionID
	exe, _ := os.Executable()

	exec.Command("tmux", "set-option", "-t", tmuxName, "status-right",
		" ctrl-b d: detach | ctrl-b n: rename ").Run()

	renameTemplate := fmt.Sprintf(
		`run-shell '%s session rename %s "%%%%" >/dev/null 2>&1 && tmux display-message "Session renamed" || tmux display-message "Rename failed"'`,
		exe, sessionID)
	exec.Command("tmux", "bind-key", "-T", "prefix", "n",
		"command-prompt", "-I", sessionName, "-p", "Session name:", renameTemplate).Run()

	c := exec.Command("tmux", "attach-session", "-t", tmuxName)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return tmuxDetachedMsg{err: err}
	})
}

func detachSessionCmd(conn protocol.Conn) tea.Cmd {
	return func() tea.Msg {
		req, _ := protocol.NewRequest(protocol.MsgSessionDetach, "dash-detach", nil)
		conn.Send(req)
		return nil
	}
}

func renameSessionCmd(conn protocol.Conn, sessionID, name string) tea.Cmd {
	return func() tea.Msg {
		req, _ := protocol.NewRequest(protocol.MsgSessionRename, "dash-rename", protocol.SessionRenamePayload{
			SessionID: sessionID,
			Name:      name,
		})
		if err := conn.Send(req); err != nil {
			return statusMsg{text: "Error: " + err.Error()}
		}
		resp, err := conn.Receive()
		if err != nil {
			return statusMsg{text: "Error: " + err.Error()}
		}
		if resp.Type == protocol.MsgError {
			var errPayload map[string]string
			if err := json.Unmarshal(resp.Payload, &errPayload); err != nil {
				return statusMsg{text: "Rename failed"}
			}
			return statusMsg{text: "Error: " + errPayload["error"]}
		}
		return sessionRenamedMsg{sessionID: sessionID, name: name}
	}
}

func listenAttachOutputCmd(conn protocol.Conn) tea.Cmd {
	return func() tea.Msg {
		msg, err := conn.Receive()
		if err != nil {
			return attachNonOutputMsg{}
		}
		if msg.Type == protocol.MsgSessionOutput {
			var payload protocol.SessionOutputPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return attachNonOutputMsg{}
			}
			return sessionOutputMsg{sessionID: payload.SessionID, data: payload.Data}
		}
		return attachNonOutputMsg{}
	}
}

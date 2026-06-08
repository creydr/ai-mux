package dashboard

import (
	"encoding/json"
	"os/exec"
	"runtime"

	tea "charm.land/bubbletea/v2"
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
		json.Unmarshal(issueResp.Payload, &issues)

		prMsg, _ := protocol.NewRequest(protocol.MsgListPRs, "dash-prs", protocol.ListPayload{Limit: limit})
		if err := conn.Send(prMsg); err != nil {
			return errMsg{err: err}
		}
		prResp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		var prs protocol.ItemsPayload
		json.Unmarshal(prResp.Payload, &prs)

		return itemsReceivedMsg{
			issues: issues.Items,
			prs:    prs.Items,
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
		json.Unmarshal(resp.Payload, &payload)

		return repoExpandedMsg{
			repo:           repo,
			items:          payload.Items,
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
			json.Unmarshal(msg.Payload, &ev)
			return eventReceivedMsg{event: ev}
		}
		return nil
	}
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		cmd.Run()
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
		json.Unmarshal(resp.Payload, &payload)
		return sessionsReceivedMsg{sessions: payload.Sessions}
	}
}

func spawnSessionCmd(conn protocol.Conn, repo string, number int, itemType, agent string) tea.Cmd {
	return func() tea.Msg {
		req, _ := protocol.NewRequest(protocol.MsgSessionSpawn, "dash-spawn", protocol.SessionSpawnPayload{
			Repo:     repo,
			Number:   number,
			ItemType: itemType,
			Agent:    agent,
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
			json.Unmarshal(resp.Payload, &errPayload)
			return statusMsg{text: "Error: " + errPayload["error"]}
		}
		var sess protocol.SessionPayload
		json.Unmarshal(resp.Payload, &sess)
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
			json.Unmarshal(resp.Payload, &errPayload)
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
			json.Unmarshal(resp.Payload, &errPayload)
			return statusMsg{text: "Error: " + errPayload["error"]}
		}
		return sessionAttachedMsg{}
	}
}

func detachSessionCmd(conn protocol.Conn) tea.Cmd {
	return func() tea.Msg {
		req, _ := protocol.NewRequest(protocol.MsgSessionDetach, "dash-detach", nil)
		conn.Send(req)
		return nil
	}
}

func sendInputCmd(conn protocol.Conn, sessionID, input string) tea.Cmd {
	return func() tea.Msg {
		req, _ := protocol.NewRequest(protocol.MsgSessionInput, "dash-input", protocol.SessionInputPayload{
			SessionID: sessionID,
			Input:     input,
		})
		conn.Send(req)
		return nil
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
			json.Unmarshal(msg.Payload, &payload)
			return sessionOutputMsg{sessionID: payload.SessionID, data: payload.Data}
		}
		return attachNonOutputMsg{}
	}
}

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

package attach

import (
	"encoding/json"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/tui"
)

func fetchItemCmd(conn protocol.Conn, ref Ref) tea.Cmd {
	return func() tea.Msg {
		resp, err := protocol.SendRequest(conn, protocol.MsgGetItem, "attach-item", protocol.GetItemPayload{
			Repo:   ref.Owner + "/" + ref.Repo,
			Type:   string(ref.Type),
			Number: ref.Number,
		}, protocol.DefaultTimeout)
		if err != nil {
			return tui.ErrMsg{Err: err}
		}
		var item provider.Item
		if err := json.Unmarshal(resp.Payload, &item); err != nil {
			return tui.ErrMsg{Err: fmt.Errorf("parsing item: %w", err)}
		}
		return itemLoadedMsg{item: &item}
	}
}

func fetchJiraItemDetailCmd(conn protocol.Conn, key string) tea.Cmd {
	return func() tea.Msg {
		resp, err := protocol.SendRequest(conn, protocol.MsgGetJiraItem, "attach-jira-item", protocol.JiraKeyPayload{
			Key: key,
		}, protocol.DefaultTimeout)
		if err != nil {
			return tui.ErrMsg{Err: err}
		}
		var item provider.JiraItem
		if err := json.Unmarshal(resp.Payload, &item); err != nil {
			return tui.ErrMsg{Err: fmt.Errorf("parsing jira item: %w", err)}
		}

		commResp, err := protocol.SendRequest(conn, protocol.MsgGetJiraComments, "attach-jira-comments", protocol.JiraKeyPayload{
			Key: key,
		}, protocol.DefaultTimeout)
		if err != nil {
			return jiraItemLoadedMsg{item: &item}
		}
		var commPayload protocol.JiraCommentsPayload
		if err := json.Unmarshal(commResp.Payload, &commPayload); err != nil {
			return jiraItemLoadedMsg{item: &item}
		}
		var comments []provider.JiraComment
		if err := json.Unmarshal(commPayload.Comments, &comments); err != nil {
			return jiraItemLoadedMsg{item: &item}
		}

		return jiraItemLoadedMsg{item: &item, comments: comments}
	}
}

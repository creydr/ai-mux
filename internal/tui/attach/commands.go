package attach

import (
	"encoding/json"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

func fetchItemCmd(conn protocol.Conn, ref Ref) tea.Cmd {
	return func() tea.Msg {
		msg, err := protocol.NewRequest(protocol.MsgGetItem, "attach-item", protocol.GetItemPayload{
			Repo:   ref.Owner + "/" + ref.Repo,
			Type:   string(ref.Type),
			Number: ref.Number,
		})
		if err != nil {
			return errMsg{err: err}
		}
		if err := conn.Send(msg); err != nil {
			return errMsg{err: err}
		}
		resp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		var item provider.Item
		if err := json.Unmarshal(resp.Payload, &item); err != nil {
			return errMsg{err: fmt.Errorf("parsing item: %w", err)}
		}
		return itemLoadedMsg{item: &item}
	}
}

func fetchReviewsCmd(conn protocol.Conn, ref Ref) tea.Cmd {
	return func() tea.Msg {
		msg, err := protocol.NewRequest("list_reviews", "attach-reviews", protocol.GetItemPayload{
			Repo:   ref.Owner + "/" + ref.Repo,
			Number: ref.Number,
		})
		if err != nil {
			return errMsg{err: err}
		}
		if err := conn.Send(msg); err != nil {
			return errMsg{err: err}
		}
		resp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		var reviews []provider.Review
		if err := json.Unmarshal(resp.Payload, &reviews); err != nil {
			return errMsg{err: fmt.Errorf("parsing reviews: %w", err)}
		}
		return reviewsLoadedMsg{reviews: reviews}
	}
}

func fetchCommentsCmd(conn protocol.Conn, ref Ref) tea.Cmd {
	return func() tea.Msg {
		msg, err := protocol.NewRequest("list_comments", "attach-comments", protocol.GetItemPayload{
			Repo:   ref.Owner + "/" + ref.Repo,
			Number: ref.Number,
		})
		if err != nil {
			return errMsg{err: err}
		}
		if err := conn.Send(msg); err != nil {
			return errMsg{err: err}
		}
		resp, err := conn.Receive()
		if err != nil {
			return errMsg{err: err}
		}
		var comments []provider.Comment
		if err := json.Unmarshal(resp.Payload, &comments); err != nil {
			return errMsg{err: fmt.Errorf("parsing comments: %w", err)}
		}
		return commentsLoadedMsg{comments: comments}
	}
}

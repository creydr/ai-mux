package attach

import (
	"encoding/json"

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
		json.Unmarshal(resp.Payload, &item)
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
		json.Unmarshal(resp.Payload, &reviews)
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
		json.Unmarshal(resp.Payload, &comments)
		return commentsLoadedMsg{comments: comments}
	}
}

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

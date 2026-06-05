package protocol

import "encoding/json"

type MessageType string

const (
	MsgSubscribe     MessageType = "subscribe"
	MsgListIssues    MessageType = "list_issues"
	MsgListPRs       MessageType = "list_prs"
	MsgGetItem       MessageType = "get_item"
	MsgMarkRead      MessageType = "mark_read"
	MsgExecuteAction MessageType = "execute_action"
	MsgGetStatus     MessageType = "get_status"

	MsgEvent        MessageType = "event"
	MsgStateUpdate  MessageType = "state_update"
	MsgActionResult MessageType = "action_result"
	MsgResponse     MessageType = "response"
	MsgError        MessageType = "error"
)

type Message struct {
	Type    MessageType     `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

type Conn interface {
	Send(msg Message) error
	Receive() (Message, error)
	Close() error
}

type Listener interface {
	Accept() (Conn, error)
	Close() error
	Addr() string
}

type Transport interface {
	Listen(addr string) (Listener, error)
	Dial(addr string) (Conn, error)
}

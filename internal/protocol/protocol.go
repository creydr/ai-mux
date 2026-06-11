package protocol

import "encoding/json"

type MessageType string

const (
	MsgSubscribe  MessageType = "subscribe"
	MsgListIssues MessageType = "list_issues"
	MsgListPRs    MessageType = "list_prs"
	MsgGetItem    MessageType = "get_item"
	MsgMarkRead   MessageType = "mark_read"
	MsgGetStatus  MessageType = "get_status"

	MsgSessionSpawn     MessageType = "session_spawn"
	MsgSessionList      MessageType = "session_list"
	MsgSessionAttach    MessageType = "session_attach"
	MsgSessionDetach    MessageType = "session_detach"
	MsgSessionInput     MessageType = "session_input"
	MsgSessionStop      MessageType = "session_stop"
	MsgSessionRename    MessageType = "session_rename"
	MsgSessionTypeInput MessageType = "session.type_input"

	MsgWorktreeExists MessageType = "worktree_exists"

	MsgListJiraItems   MessageType = "list_jira_items"
	MsgGetJiraItem     MessageType = "get_jira_item"
	MsgGetJiraComments MessageType = "get_jira_comments"

	MsgEvent         MessageType = "event"
	MsgSessionOutput MessageType = "session_output"
	MsgResponse      MessageType = "response"
	MsgError         MessageType = "error"
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

package acp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
)

type Agent struct {
	server *Server
	conn   protocol.Conn
	socket string
}

func NewAgent(reader io.Reader, writer io.Writer, socket string) *Agent {
	a := &Agent{
		server: NewServer(reader, writer),
		socket: socket,
	}
	a.registerHandlers()
	return a
}

func (a *Agent) Serve() error {
	transport := jsonlines.NewTransport()
	conn, err := transport.Dial(a.socket)
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w", err)
	}
	a.conn = conn
	defer conn.Close()

	return a.server.Serve()
}

func (a *Agent) registerHandlers() {
	a.server.Handle("initialize", a.handleInitialize)
	a.server.Handle("session/new", a.handleSessionNew)
	a.server.Handle("session/prompt", a.handleSessionPrompt)
	a.server.Handle("items/list", a.handleItemsList)
}

func (a *Agent) handleInitialize(params json.RawMessage) (any, error) {
	return InitializeResult{
		ServerInfo: ServerInfo{
			Name:    "ai-mux",
			Version: "0.1.0",
		},
		Capabilities: Capabilities{
			Sessions: true,
			Diffs:    true,
		},
	}, nil
}

func (a *Agent) handleSessionNew(params json.RawMessage) (any, error) {
	var p SessionNewParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	return SessionNewResult{
		SessionID: fmt.Sprintf("session-%s", p.ItemRef),
		Worktree:  fmt.Sprintf("/tmp/ai-mux/worktrees/%s", p.ItemRef),
	}, nil
}

func (a *Agent) handleSessionPrompt(params json.RawMessage) (any, error) {
	var p SessionPromptParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	return SessionPromptResult{
		Status:  "accepted",
		Message: fmt.Sprintf("Prompt queued for session %s", p.SessionID),
	}, nil
}

func (a *Agent) handleItemsList(params json.RawMessage) (any, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("not connected to daemon")
	}

	msg, err := protocol.NewRequest(protocol.MsgListIssues, "acp-list", protocol.ListPayload{})
	if err != nil {
		return nil, err
	}
	if err := a.conn.Send(msg); err != nil {
		return nil, err
	}
	resp, err := a.conn.Receive()
	if err != nil {
		return nil, err
	}

	var items protocol.ItemsPayload
	json.Unmarshal(resp.Payload, &items)
	return items, nil
}

package acp

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"

	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
)

type Agent struct {
	server *Server
	conn   protocol.Conn
	socket string
	reqID  atomic.Int64
}

func (a *Agent) nextID() string {
	return strconv.FormatInt(a.reqID.Add(1), 10)
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
	a.server.Handle("session/list", a.handleSessionList)
	a.server.Handle("session/stop", a.handleSessionStop)
	a.server.Handle("session/prompt", a.handleSessionPrompt)
	a.server.Handle("session/attach", a.handleSessionAttach)
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
	if a.conn == nil {
		return nil, fmt.Errorf("not connected to daemon")
	}

	var p SessionNewParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	msg, err := protocol.NewRequest(protocol.MsgSessionSpawn, a.nextID(), protocol.SessionSpawnPayload{
		Repo:     p.Repo,
		Number:   p.Number,
		ItemType: p.ItemType,
		Agent:    p.Agent,
	})
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
	if resp.Type == protocol.MsgError {
		var errPayload map[string]string
		if err := json.Unmarshal(resp.Payload, &errPayload); err != nil {
			return nil, fmt.Errorf("daemon error (unparseable response)")
		}
		return nil, fmt.Errorf("%s", errPayload["error"])
	}

	var sess protocol.SessionPayload
	if err := json.Unmarshal(resp.Payload, &sess); err != nil {
		return nil, fmt.Errorf("parsing session response: %w", err)
	}

	return SessionNewResult{
		SessionID: sess.ID,
		Worktree:  sess.Worktree,
	}, nil
}

func (a *Agent) handleSessionList(params json.RawMessage) (any, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("not connected to daemon")
	}

	msg, err := protocol.NewRequest(protocol.MsgSessionList, a.nextID(), nil)
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

	var payload protocol.SessionListPayload
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		return nil, fmt.Errorf("parsing session list: %w", err)
	}

	sessions := make([]SessionInfo, len(payload.Sessions))
	for i, s := range payload.Sessions {
		sessions[i] = SessionInfo{
			ID:           s.ID,
			Repo:         s.Repo,
			Number:       s.Number,
			ItemType:     s.ItemType,
			Agent:        s.Agent,
			Status:       s.Status,
			WaitingInput: s.WaitingInput,
			Worktree:     s.Worktree,
			CreatedAt:    s.CreatedAt,
		}
	}

	return SessionListResult{Sessions: sessions}, nil
}

func (a *Agent) handleSessionStop(params json.RawMessage) (any, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("not connected to daemon")
	}

	var p SessionStopParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	msg, err := protocol.NewRequest(protocol.MsgSessionStop, a.nextID(), protocol.SessionIDPayload{
		SessionID: p.SessionID,
	})
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
	if resp.Type == protocol.MsgError {
		var errPayload map[string]string
		if err := json.Unmarshal(resp.Payload, &errPayload); err != nil {
			return nil, fmt.Errorf("daemon error (unparseable response)")
		}
		return nil, fmt.Errorf("%s", errPayload["error"])
	}

	return SessionStopResult{Status: "stopped"}, nil
}

func (a *Agent) handleSessionPrompt(params json.RawMessage) (any, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("not connected to daemon")
	}

	var p SessionPromptParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	msg, err := protocol.NewRequest(protocol.MsgSessionInput, a.nextID(), protocol.SessionInputPayload{
		SessionID: p.SessionID,
		Input:     p.Prompt,
	})
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
	if resp.Type == protocol.MsgError {
		var errPayload map[string]string
		if err := json.Unmarshal(resp.Payload, &errPayload); err != nil {
			return nil, fmt.Errorf("daemon error (unparseable response)")
		}
		return nil, fmt.Errorf("%s", errPayload["error"])
	}

	return SessionPromptResult{
		Status:  "accepted",
		Message: fmt.Sprintf("Input sent to session %s", p.SessionID),
	}, nil
}

func (a *Agent) handleSessionAttach(params json.RawMessage) (any, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("not connected to daemon")
	}

	var p SessionAttachParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	msg, err := protocol.NewRequest(protocol.MsgSessionAttach, a.nextID(), protocol.SessionIDPayload{
		SessionID: p.SessionID,
	})
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
	if resp.Type == protocol.MsgError {
		var errPayload map[string]string
		if err := json.Unmarshal(resp.Payload, &errPayload); err != nil {
			return nil, fmt.Errorf("daemon error (unparseable response)")
		}
		return nil, fmt.Errorf("%s", errPayload["error"])
	}

	go a.streamOutput()

	return SessionAttachResult{Status: "attached"}, nil
}

func (a *Agent) streamOutput() {
	for {
		msg, err := a.conn.Receive()
		if err != nil {
			return
		}
		if msg.Type == protocol.MsgSessionOutput {
			var payload protocol.SessionOutputPayload
			json.Unmarshal(msg.Payload, &payload)
			a.server.WriteNotification("session/output", SessionOutputNotification{
				SessionID: payload.SessionID,
				Data:      payload.Data,
			})
		}
	}
}

func (a *Agent) handleItemsList(params json.RawMessage) (any, error) {
	if a.conn == nil {
		return nil, fmt.Errorf("not connected to daemon")
	}

	msg, err := protocol.NewRequest(protocol.MsgListIssues, a.nextID(), protocol.ListPayload{})
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
	if err := json.Unmarshal(resp.Payload, &items); err != nil {
		return nil, fmt.Errorf("parsing items list: %w", err)
	}
	return items, nil
}

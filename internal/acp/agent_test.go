package acp

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/protocol/jsonlines"
)

func startMockDaemon(t *testing.T, handler func(protocol.Conn)) string {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "test.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		conn := jsonlines.WrapConn(raw)
		defer conn.Close()
		handler(conn)
	}()

	return sock
}

func runAgent(t *testing.T, sock, input string) []Response {
	t.Helper()
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	agent := NewAgent(reader, &writer, sock)
	agent.Serve()

	var responses []Response
	for _, line := range strings.Split(strings.TrimSpace(writer.String()), "\n") {
		if line == "" {
			continue
		}
		var resp Response
		json.Unmarshal([]byte(line), &resp)
		responses = append(responses, resp)
	}
	return responses
}

func TestAgent_Initialize(t *testing.T) {
	sock := startMockDaemon(t, func(conn protocol.Conn) {})

	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"vscode","version":"1.0"}}}` + "\n"
	responses := runAgent(t, sock, input)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result InitializeResult
	json.Unmarshal(data, &result)

	if result.ServerInfo.Name != "ai-mux" {
		t.Errorf("expected server name ai-mux, got %q", result.ServerInfo.Name)
	}
	if !result.Capabilities.Sessions {
		t.Error("expected sessions capability")
	}
}

func TestAgent_SessionNew(t *testing.T) {
	sock := startMockDaemon(t, func(conn protocol.Conn) {
		msg, _ := conn.Receive()
		if msg.Type != protocol.MsgSessionSpawn {
			t.Errorf("expected MsgSessionSpawn, got %s", msg.Type)
		}
		var payload protocol.SessionSpawnPayload
		json.Unmarshal(msg.Payload, &payload)
		if payload.Repo != "owner/repo" || payload.Number != 42 {
			t.Errorf("unexpected payload: %+v", payload)
		}

		resp, _ := protocol.NewResponse(msg.ID, protocol.SessionPayload{
			ID:       "fix-42-abcd",
			Repo:     "owner/repo",
			Number:   42,
			ItemType: "issue",
			Agent:    "claude",
			Status:   "running",
			Worktree: "/tmp/wt/fix-42",
		})
		conn.Send(resp)
	})

	input := `{"jsonrpc":"2.0","id":1,"method":"session/new","params":{"repo":"owner/repo","number":42,"itemType":"issue","agent":"claude"}}` + "\n"
	responses := runAgent(t, sock, input)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result SessionNewResult
	json.Unmarshal(data, &result)

	if result.SessionID != "fix-42-abcd" {
		t.Errorf("expected session ID fix-42-abcd, got %q", result.SessionID)
	}
	if result.Worktree != "/tmp/wt/fix-42" {
		t.Errorf("expected worktree /tmp/wt/fix-42, got %q", result.Worktree)
	}
}

func TestAgent_SessionList(t *testing.T) {
	sock := startMockDaemon(t, func(conn protocol.Conn) {
		msg, _ := conn.Receive()
		if msg.Type != protocol.MsgSessionList {
			t.Errorf("expected MsgSessionList, got %s", msg.Type)
		}

		resp, _ := protocol.NewResponse(msg.ID, protocol.SessionListPayload{
			Sessions: []protocol.SessionPayload{
				{ID: "fix-1-aaaa", Repo: "o/r", Number: 1, Agent: "claude", Status: "running"},
				{ID: "rev-2-bbbb", Repo: "o/r", Number: 2, Agent: "claude", Status: "running"},
			},
		})
		conn.Send(resp)
	})

	input := `{"jsonrpc":"2.0","id":1,"method":"session/list","params":{}}` + "\n"
	responses := runAgent(t, sock, input)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result SessionListResult
	json.Unmarshal(data, &result)

	if len(result.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(result.Sessions))
	}
	if result.Sessions[0].ID != "fix-1-aaaa" {
		t.Errorf("expected first session ID fix-1-aaaa, got %q", result.Sessions[0].ID)
	}
}

func TestAgent_SessionStop(t *testing.T) {
	sock := startMockDaemon(t, func(conn protocol.Conn) {
		msg, _ := conn.Receive()
		if msg.Type != protocol.MsgSessionStop {
			t.Errorf("expected MsgSessionStop, got %s", msg.Type)
		}
		var payload protocol.SessionIDPayload
		json.Unmarshal(msg.Payload, &payload)
		if payload.SessionID != "fix-42-abcd" {
			t.Errorf("expected session ID fix-42-abcd, got %q", payload.SessionID)
		}

		resp, _ := protocol.NewResponse(msg.ID, map[string]string{"status": "stopped"})
		conn.Send(resp)
	})

	input := `{"jsonrpc":"2.0","id":1,"method":"session/stop","params":{"sessionId":"fix-42-abcd"}}` + "\n"
	responses := runAgent(t, sock, input)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result SessionStopResult
	json.Unmarshal(data, &result)

	if result.Status != "stopped" {
		t.Errorf("expected status stopped, got %q", result.Status)
	}
}

func TestAgent_SessionPrompt(t *testing.T) {
	sock := startMockDaemon(t, func(conn protocol.Conn) {
		msg, _ := conn.Receive()
		if msg.Type != protocol.MsgSessionInput {
			t.Errorf("expected MsgSessionInput, got %s", msg.Type)
		}
		var payload protocol.SessionInputPayload
		json.Unmarshal(msg.Payload, &payload)
		if payload.SessionID != "fix-42-abcd" {
			t.Errorf("expected session ID fix-42-abcd, got %q", payload.SessionID)
		}
		if payload.Input != "fix the bug" {
			t.Errorf("expected input 'fix the bug', got %q", payload.Input)
		}

		resp, _ := protocol.NewResponse(msg.ID, map[string]string{"status": "sent"})
		conn.Send(resp)
	})

	input := `{"jsonrpc":"2.0","id":1,"method":"session/prompt","params":{"sessionId":"fix-42-abcd","prompt":"fix the bug"}}` + "\n"
	responses := runAgent(t, sock, input)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result SessionPromptResult
	json.Unmarshal(data, &result)

	if result.Status != "accepted" {
		t.Errorf("expected status accepted, got %q", result.Status)
	}
}

func TestAgent_ItemsList(t *testing.T) {
	sock := startMockDaemon(t, func(conn protocol.Conn) {
		msg, _ := conn.Receive()
		if msg.Type != protocol.MsgListIssues {
			t.Errorf("expected MsgListIssues, got %s", msg.Type)
		}

		resp, _ := protocol.NewResponse(msg.ID, protocol.ItemsPayload{})
		conn.Send(resp)
	})

	input := `{"jsonrpc":"2.0","id":1,"method":"items/list","params":{}}` + "\n"
	responses := runAgent(t, sock, input)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("unexpected error: %v", responses[0].Error)
	}
}

// WrapConn is tested here to ensure the jsonlines package exposes it
func init() {
	_ = os.DevNull
}

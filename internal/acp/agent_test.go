package acp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestAgent_Initialize(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"vscode","version":"1.0"}}}` + "\n"
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	agent := NewAgent(reader, &writer, "/tmp/nonexistent.sock")
	agent.server.Serve()

	var resp Response
	json.Unmarshal(writer.Bytes(), &resp)

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
	if !result.Capabilities.Diffs {
		t.Error("expected diffs capability")
	}
}

func TestAgent_SessionNew(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"session/new","params":{"itemRef":"pr-owner-repo-42","agent":"claude"}}` + "\n"
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	agent := NewAgent(reader, &writer, "/tmp/nonexistent.sock")
	agent.server.Serve()

	var resp Response
	json.Unmarshal(writer.Bytes(), &resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result SessionNewResult
	json.Unmarshal(data, &result)

	if result.SessionID == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestAgent_SessionPrompt(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"session/prompt","params":{"sessionId":"s1","prompt":"review the changes"}}` + "\n"
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	agent := NewAgent(reader, &writer, "/tmp/nonexistent.sock")
	agent.server.Serve()

	var resp Response
	json.Unmarshal(writer.Bytes(), &resp)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result SessionPromptResult
	json.Unmarshal(data, &result)

	if result.Status != "accepted" {
		t.Errorf("expected status 'accepted', got %q", result.Status)
	}
}

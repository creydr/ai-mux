package acp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestServer_Initialize(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	s := NewServer(reader, &writer)
	s.Handle("initialize", func(params json.RawMessage) (any, error) {
		return map[string]string{"status": "ok"}, nil
	})

	s.Serve()

	var resp Response
	json.Unmarshal(writer.Bytes(), &resp)

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %q", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got %v", resp.Error)
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"nonexistent"}` + "\n"
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	s := NewServer(reader, &writer)
	s.Serve()

	var resp Response
	json.Unmarshal(writer.Bytes(), &resp)

	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601, got %d", resp.Error.Code)
	}
}

func TestServer_InvalidJSON(t *testing.T) {
	input := `not valid json` + "\n"
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	s := NewServer(reader, &writer)
	s.Serve()

	var resp Response
	json.Unmarshal(writer.Bytes(), &resp)

	if resp.Error == nil {
		t.Fatal("expected parse error")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected code -32700, got %d", resp.Error.Code)
	}
}

func TestServer_InvalidVersion(t *testing.T) {
	input := `{"jsonrpc":"1.0","id":1,"method":"test"}` + "\n"
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	s := NewServer(reader, &writer)
	s.Serve()

	var resp Response
	json.Unmarshal(writer.Bytes(), &resp)

	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("expected code -32600, got %d", resp.Error.Code)
	}
}

func TestServer_MultipleRequests(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"echo","params":"hello"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"echo","params":"world"}` + "\n"
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	s := NewServer(reader, &writer)
	s.Handle("echo", func(params json.RawMessage) (any, error) {
		var s string
		json.Unmarshal(params, &s)
		return s, nil
	})

	s.Serve()

	lines := strings.Split(strings.TrimSpace(writer.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(lines))
	}
}

func TestServer_HandlerError(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"fail"}` + "\n"
	reader := strings.NewReader(input)
	var writer bytes.Buffer

	s := NewServer(reader, &writer)
	s.Handle("fail", func(params json.RawMessage) (any, error) {
		return nil, fmt.Errorf("something went wrong")
	})

	s.Serve()

	var resp Response
	json.Unmarshal(writer.Bytes(), &resp)

	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32000 {
		t.Errorf("expected code -32000, got %d", resp.Error.Code)
	}
}

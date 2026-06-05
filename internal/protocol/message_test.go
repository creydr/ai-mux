package protocol

import (
	"testing"
)

func TestNewRequest_RoundTrip(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	msg, err := NewRequest(MsgListIssues, "req-1", payload{Name: "test", Count: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.Type != MsgListIssues {
		t.Errorf("expected type %s, got %s", MsgListIssues, msg.Type)
	}
	if msg.ID != "req-1" {
		t.Errorf("expected id req-1, got %s", msg.ID)
	}

	result, err := ParsePayload[payload](msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("expected name test, got %s", result.Name)
	}
	if result.Count != 42 {
		t.Errorf("expected count 42, got %d", result.Count)
	}
}

func TestNewResponse_RoundTrip(t *testing.T) {
	type payload struct {
		Status string `json:"status"`
	}

	msg, err := NewResponse("req-1", payload{Status: "ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.Type != MsgResponse {
		t.Errorf("expected type %s, got %s", MsgResponse, msg.Type)
	}
	if msg.ID != "req-1" {
		t.Errorf("expected id req-1, got %s", msg.ID)
	}

	result, err := ParsePayload[payload](msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("expected status ok, got %s", result.Status)
	}
}

func TestNewError(t *testing.T) {
	msg, err := NewError("req-1", "something failed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.Type != MsgError {
		t.Errorf("expected type %s, got %s", MsgError, msg.Type)
	}

	result, err := ParsePayload[map[string]string](msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["error"] != "something failed" {
		t.Errorf("expected error message 'something failed', got %s", result["error"])
	}
}

func TestParsePayload_InvalidJSON(t *testing.T) {
	msg := Message{
		Type:    MsgResponse,
		Payload: []byte("not json"),
	}

	_, err := ParsePayload[map[string]string](msg)
	if err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}

func TestNewRequest_EmptyPayload(t *testing.T) {
	msg, err := NewRequest(MsgGetStatus, "req-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg.Payload) != "null" {
		t.Errorf("expected null payload, got %s", string(msg.Payload))
	}
}

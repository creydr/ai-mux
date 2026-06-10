package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type mockConn struct {
	sent    []Message
	recvMsg Message
	recvErr error
	block   bool
}

func (m *mockConn) Send(msg Message) error {
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockConn) Receive() (Message, error) {
	if m.block {
		select {}
	}
	return m.recvMsg, m.recvErr
}

func (m *mockConn) Close() error { return nil }

func TestSendRequest_Success(t *testing.T) {
	respPayload, _ := json.Marshal(map[string]string{"status": "ok"})
	mc := &mockConn{
		recvMsg: Message{Type: MsgResponse, Payload: respPayload},
	}

	resp, err := SendRequest(mc, MsgSessionList, "test-id", nil, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Type != MsgResponse {
		t.Errorf("expected MsgResponse, got %s", resp.Type)
	}
	if len(mc.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(mc.sent))
	}
	if mc.sent[0].Type != MsgSessionList {
		t.Errorf("sent wrong message type: %s", mc.sent[0].Type)
	}
}

func TestSendRequest_Timeout(t *testing.T) {
	mc := &mockConn{block: true}

	_, err := SendRequest(mc, MsgSessionList, "test-id", nil, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout message, got: %v", err)
	}
}

func TestSendRequest_MarshalError(t *testing.T) {
	mc := &mockConn{}

	_, err := SendRequest(mc, MsgSessionList, "test-id", make(chan int), time.Second)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "creating request") {
		t.Errorf("expected creating request error, got: %v", err)
	}
}

func TestSendRequest_ReceiveError(t *testing.T) {
	mc := &mockConn{recvErr: fmt.Errorf("connection closed")}

	_, err := SendRequest(mc, MsgSessionList, "test-id", nil, time.Second)
	if err == nil {
		t.Fatal("expected receive error")
	}
	if !strings.Contains(err.Error(), "connection closed") {
		t.Errorf("expected connection closed error, got: %v", err)
	}
}

func TestParseErrorPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{"valid", `{"error":"not found"}`, "not found"},
		{"missing key", `{"status":"fail"}`, ""},
		{"invalid json", `{broken`, "unknown error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{Payload: json.RawMessage(tt.payload)}
			got := ParseErrorPayload(msg)
			if got != tt.want {
				t.Errorf("ParseErrorPayload() = %q, want %q", got, tt.want)
			}
		})
	}
}

package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

const DefaultTimeout = 30 * time.Second

// SendRequest creates a request, sends it, and waits for a response with a timeout.
// Long-polling commands (event listeners, output streams) should use conn.Receive() directly.
func SendRequest(conn Conn, msgType MessageType, id string, payload any, timeout time.Duration) (Message, error) {
	req, err := NewRequest(msgType, id, payload)
	if err != nil {
		return Message{}, fmt.Errorf("creating request: %w", err)
	}
	if err := conn.Send(req); err != nil {
		return Message{}, fmt.Errorf("sending request: %w", err)
	}
	type result struct {
		msg Message
		err error
	}
	ch := make(chan result, 1)
	go func() {
		msg, err := conn.Receive()
		ch <- result{msg, err}
	}()
	select {
	case r := <-ch:
		return r.msg, r.err
	case <-time.After(timeout):
		return Message{}, fmt.Errorf("request %s timed out after %v", msgType, timeout)
	}
}

func ParseErrorPayload(msg Message) string {
	var payload map[string]string
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return "unknown error"
	}
	return payload["error"]
}

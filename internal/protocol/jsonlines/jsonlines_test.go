package jsonlines

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/creydr/ai-mux/internal/protocol"
)

func TestTransport_ListenDial(t *testing.T) {
	addr := tempSocket(t)
	transport := NewTransport()

	ln, err := transport.Listen(addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	var serverConn protocol.Conn
	var acceptErr error
	accepted := make(chan struct{})
	go func() {
		serverConn, acceptErr = ln.Accept()
		close(accepted)
	}()

	clientConn, err := transport.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer clientConn.Close()

	<-accepted
	if acceptErr != nil {
		t.Fatalf("accept: %v", acceptErr)
	}
	defer serverConn.Close()
}

func TestConn_SendReceive(t *testing.T) {
	serverConn, clientConn := connPair(t)

	msg := protocol.Message{
		Type:    protocol.MsgGetStatus,
		ID:      "req-1",
		Payload: json.RawMessage(`{"running":true}`),
	}

	if err := clientConn.Send(msg); err != nil {
		t.Fatalf("send: %v", err)
	}

	received, err := serverConn.Receive()
	if err != nil {
		t.Fatalf("receive: %v", err)
	}

	if received.Type != protocol.MsgGetStatus {
		t.Errorf("expected type %s, got %s", protocol.MsgGetStatus, received.Type)
	}
	if received.ID != "req-1" {
		t.Errorf("expected id req-1, got %s", received.ID)
	}
}

func TestConn_MultipleMessages(t *testing.T) {
	serverConn, clientConn := connPair(t)

	for i := range 5 {
		msg := protocol.Message{
			Type:    protocol.MsgListIssues,
			ID:      string(rune('0' + i)),
			Payload: json.RawMessage(`{}`),
		}
		if err := clientConn.Send(msg); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	for i := range 5 {
		received, err := serverConn.Receive()
		if err != nil {
			t.Fatalf("receive %d: %v", i, err)
		}
		if received.ID != string(rune('0'+i)) {
			t.Errorf("message %d: expected id %s, got %s", i, string(rune('0'+i)), received.ID)
		}
	}
}

func TestConn_Close(t *testing.T) {
	serverConn, clientConn := connPair(t)

	clientConn.Close()

	_, err := serverConn.Receive()
	if err == nil {
		t.Fatal("expected error after client close")
	}
}

func TestListener_MultipleClients(t *testing.T) {
	addr := tempSocket(t)
	transport := NewTransport()

	ln, err := transport.Listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var wg sync.WaitGroup
	serverConns := make([]protocol.Conn, 2)

	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, err := ln.Accept()
			if err != nil {
				t.Errorf("accept %d: %v", idx, err)
				return
			}
			serverConns[idx] = conn
		}(i)
	}

	client1, err := transport.Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client1.Close()

	client2, err := transport.Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client2.Close()

	wg.Wait()

	msg1 := protocol.Message{Type: protocol.MsgListIssues, ID: "c1", Payload: json.RawMessage(`{}`)}
	msg2 := protocol.Message{Type: protocol.MsgListPRs, ID: "c2", Payload: json.RawMessage(`{}`)}

	client1.Send(msg1)
	client2.Send(msg2)

	r1, _ := serverConns[0].Receive()
	r2, _ := serverConns[1].Receive()

	ids := map[string]bool{r1.ID: true, r2.ID: true}
	if !ids["c1"] || !ids["c2"] {
		t.Errorf("expected both c1 and c2, got %v and %v", r1.ID, r2.ID)
	}

	for _, sc := range serverConns {
		if sc != nil {
			sc.Close()
		}
	}
}

func TestListener_Close(t *testing.T) {
	addr := tempSocket(t)
	transport := NewTransport()

	ln, err := transport.Listen(addr)
	if err != nil {
		t.Fatal(err)
	}

	ln.Close()

	_, err = ln.Accept()
	if err == nil {
		t.Fatal("expected error after listener close")
	}
}

func TestTransport_SocketCleanup(t *testing.T) {
	addr := tempSocket(t)
	transport := NewTransport()

	ln, err := transport.Listen(addr)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(addr); os.IsNotExist(err) {
		t.Fatal("socket file should exist while listener is open")
	}

	ln.Close()

	if _, err := os.Stat(addr); !os.IsNotExist(err) {
		t.Fatal("socket file should be removed after listener close")
	}
}

func TestListener_Addr(t *testing.T) {
	addr := tempSocket(t)
	transport := NewTransport()

	ln, err := transport.Listen(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	if ln.Addr() != addr {
		t.Errorf("expected addr %s, got %s", addr, ln.Addr())
	}
}

func tempSocket(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test.sock")
}

func connPair(t *testing.T) (protocol.Conn, protocol.Conn) {
	t.Helper()
	addr := tempSocket(t)
	transport := NewTransport()

	ln, err := transport.Listen(addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	var serverConn protocol.Conn
	var acceptErr error
	accepted := make(chan struct{})
	go func() {
		serverConn, acceptErr = ln.Accept()
		close(accepted)
	}()

	clientConn, err := transport.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { clientConn.Close() })

	<-accepted
	if acceptErr != nil {
		t.Fatalf("accept: %v", acceptErr)
	}
	t.Cleanup(func() { serverConn.Close() })

	return serverConn, clientConn
}

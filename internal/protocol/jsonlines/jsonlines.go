package jsonlines

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/creydr/ai-mux/internal/protocol"
)

type Transport struct{}

func NewTransport() *Transport {
	return &Transport{}
}

func (t *Transport) Listen(addr string) (protocol.Listener, error) {
	_ = os.Remove(addr)

	ln, err := net.Listen("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", addr, err)
	}

	return &Listener{
		ln:   ln.(*net.UnixListener),
		addr: addr,
	}, nil
}

func (t *Transport) Dial(addr string) (protocol.Conn, error) {
	conn, err := net.Dial("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("dialing %s: %w", addr, err)
	}

	return newConn(conn), nil
}

type Listener struct {
	ln   *net.UnixListener
	addr string
}

func (l *Listener) Accept() (protocol.Conn, error) {
	conn, err := l.ln.Accept()
	if err != nil {
		return nil, err
	}
	return newConn(conn), nil
}

func (l *Listener) Close() error {
	err := l.ln.Close()
	_ = os.Remove(l.addr)
	return err
}

func (l *Listener) Addr() string {
	return l.addr
}

type Conn struct {
	conn    net.Conn
	encoder *json.Encoder
	scanner *bufio.Scanner
	wmu     sync.Mutex
	rmu     sync.Mutex
}

const maxMessageSize = 4 << 20 // 4 MB

func newConn(conn net.Conn) *Conn {
	s := bufio.NewScanner(conn)
	s.Buffer(make([]byte, 64*1024), maxMessageSize)
	return &Conn{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		scanner: s,
	}
}

func (c *Conn) Send(msg protocol.Message) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	return c.encoder.Encode(msg)
}

func (c *Conn) Receive() (protocol.Message, error) {
	c.rmu.Lock()
	defer c.rmu.Unlock()

	var msg protocol.Message
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return msg, err
		}
		return msg, io.EOF
	}
	if err := json.Unmarshal(c.scanner.Bytes(), &msg); err != nil {
		return msg, err
	}
	return msg, nil
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

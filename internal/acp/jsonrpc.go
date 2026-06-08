package acp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

type Handler func(params json.RawMessage) (any, error)

type Server struct {
	handlers map[string]Handler
	reader   io.Reader
	writer   io.Writer
}

func NewServer(reader io.Reader, writer io.Writer) *Server {
	return &Server{
		handlers: make(map[string]Handler),
		reader:   reader,
		writer:   writer,
	}
}

func (s *Server) Handle(method string, handler Handler) {
	s.handlers[method] = handler
}

func (s *Server) Serve() error {
	scanner := bufio.NewScanner(s.reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(nil, -32700, "parse error")
			continue
		}

		if req.JSONRPC != "2.0" {
			s.writeError(req.ID, -32600, "invalid request: jsonrpc must be 2.0")
			continue
		}

		handler, ok := s.handlers[req.Method]
		if !ok {
			s.writeError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
			continue
		}

		result, err := handler(req.Params)
		if err != nil {
			s.writeError(req.ID, -32000, err.Error())
			continue
		}

		s.writeResult(req.ID, result)
	}

	return scanner.Err()
}

func (s *Server) writeResult(id *json.RawMessage, result any) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("error marshaling response: %v", err)
		return
	}
	fmt.Fprintf(s.writer, "%s\n", data)
}

func (s *Server) WriteNotification(method string, params any) {
	n := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(n)
	if err != nil {
		log.Printf("error marshaling notification: %v", err)
		return
	}
	fmt.Fprintf(s.writer, "%s\n", data)
}

func (s *Server) writeError(id *json.RawMessage, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &ErrorObject{Code: code, Message: message},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("error marshaling error response: %v", err)
		return
	}
	fmt.Fprintf(s.writer, "%s\n", data)
}

package acp

import "encoding/json"

type Request struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *ErrorObject     `json:"error,omitempty"`
}

type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type ErrorObject struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeParams struct {
	ClientInfo ClientInfo `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ServerInfo   ServerInfo   `json:"serverInfo"`
	Capabilities Capabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Sessions bool `json:"sessions"`
	Diffs    bool `json:"diffs"`
}

type SessionNewParams struct {
	Repo     string `json:"repo"`
	Number   int    `json:"number"`
	ItemType string `json:"itemType"`
	Agent    string `json:"agent"`
}

type SessionNewResult struct {
	SessionID string `json:"sessionId"`
	Worktree  string `json:"worktree"`
}

type SessionListResult struct {
	Sessions []SessionInfo `json:"sessions"`
}

type SessionInfo struct {
	ID           string `json:"id"`
	Repo         string `json:"repo"`
	Number       int    `json:"number"`
	ItemType     string `json:"itemType"`
	Agent        string `json:"agent"`
	Status       string `json:"status"`
	WaitingInput bool   `json:"waitingInput"`
	Worktree     string `json:"worktree"`
	CreatedAt    string `json:"createdAt"`
}

type SessionStopParams struct {
	SessionID string `json:"sessionId"`
}

type SessionStopResult struct {
	Status string `json:"status"`
}

type SessionAttachParams struct {
	SessionID string `json:"sessionId"`
}

type SessionAttachResult struct {
	Status string `json:"status"`
}

type SessionOutputNotification struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data"`
}

type SessionPromptParams struct {
	SessionID string `json:"sessionId"`
	Prompt    string `json:"prompt"`
}

type SessionPromptResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

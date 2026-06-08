package protocol

import (
	"encoding/json"
	"fmt"
)

func NewRequest(msgType MessageType, id string, payload any) (Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("marshaling payload: %w", err)
	}
	return Message{
		Type:    msgType,
		ID:      id,
		Payload: data,
	}, nil
}

func NewResponse(id string, payload any) (Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("marshaling payload: %w", err)
	}
	return Message{
		Type:    MsgResponse,
		ID:      id,
		Payload: data,
	}, nil
}

func NewError(id string, errMsg string) (Message, error) {
	return NewRequest(MsgError, id, map[string]string{"error": errMsg})
}

func ParsePayload[T any](msg Message) (T, error) {
	var result T
	if err := json.Unmarshal(msg.Payload, &result); err != nil {
		return result, fmt.Errorf("parsing payload: %w", err)
	}
	return result, nil
}

type SubscribePayload struct {
	Repos []string `json:"repos,omitempty"`
	Types []string `json:"types,omitempty"`
}

type ListPayload struct {
	Repo  string `json:"repo"`
	Limit int    `json:"limit,omitempty"`
}

type GetItemPayload struct {
	Repo   string `json:"repo"`
	Type   string `json:"type"`
	Number int    `json:"number"`
}

type MarkReadPayload struct {
	ItemID string `json:"item_id"`
}

type ExecuteActionPayload struct {
	Action string `json:"action"`
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Agent  string `json:"agent,omitempty"`
}

type StatusPayload struct {
	Running bool     `json:"running"`
	Repos   []string `json:"repos"`
	Clients int      `json:"clients"`
	Uptime  string   `json:"uptime"`
}

type ItemsPayload struct {
	Items json.RawMessage `json:"items"`
}

type SessionSpawnPayload struct {
	Repo           string `json:"repo"`
	Number         int    `json:"number"`
	ItemType       string `json:"item_type"`
	Agent          string `json:"agent"`
	WorktreeAction string `json:"worktree_action,omitempty"`
}

type SessionPayload struct {
	ID           string `json:"id"`
	Repo         string `json:"repo"`
	Number       int    `json:"number"`
	ItemType     string `json:"item_type"`
	Agent        string `json:"agent"`
	Status       string `json:"status"`
	WaitingInput bool   `json:"waiting_input"`
	Worktree     string `json:"worktree"`
	CreatedAt    string `json:"created_at"`
}

type SessionListPayload struct {
	Sessions []SessionPayload `json:"sessions"`
}

type SessionOutputPayload struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

type SessionInputPayload struct {
	SessionID string `json:"session_id"`
	Input     string `json:"input"`
}

type SessionIDPayload struct {
	SessionID string `json:"session_id"`
}

type ActionResultPayload struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Action  string `json:"action"`
}

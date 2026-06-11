package event

import (
	"time"

	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

type Type string

const (
	TypeNewIssue        Type = "new_issue"
	TypeNewPR           Type = "new_pr"
	TypeIssueUpdated    Type = "issue_updated"
	TypePRUpdated       Type = "pr_updated"
	TypeReviewReceived  Type = "review_received"
	TypeNewComment      Type = "new_comment"
	TypeItemRead        Type = "item_read"
	TypeSessionStatus   Type = "session_status"
	TypeNewJiraItem     Type = "new_jira_item"
	TypeJiraItemUpdated Type = "jira_item_updated"
)

type Event struct {
	Type      Type                     `json:"type"`
	Item      *provider.Item           `json:"item,omitempty"`
	Review    *provider.Review         `json:"review,omitempty"`
	Comment   *provider.Comment        `json:"comment,omitempty"`
	Session   *protocol.SessionPayload `json:"session,omitempty"`
	JiraItem  *provider.JiraItem       `json:"jira_item,omitempty"`
	Timestamp time.Time                `json:"timestamp"`
}

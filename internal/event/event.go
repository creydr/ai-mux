package event

import (
	"time"

	"github.com/creydr/ai-mux/internal/provider"
)

type Type string

const (
	TypeNewIssue       Type = "new_issue"
	TypeNewPR          Type = "new_pr"
	TypeIssueUpdated   Type = "issue_updated"
	TypePRUpdated      Type = "pr_updated"
	TypeReviewReceived Type = "review_received"
	TypeNewComment     Type = "new_comment"
	TypeItemRead       Type = "item_read"
)

type Event struct {
	Type      Type              `json:"type"`
	Item      *provider.Item    `json:"item,omitempty"`
	Review    *provider.Review  `json:"review,omitempty"`
	Comment   *provider.Comment `json:"comment,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

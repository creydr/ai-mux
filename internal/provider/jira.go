package provider

import "time"

const ItemTypeJira ItemType = "jira"

type JiraItem struct {
	Key         string          `json:"key"`
	Summary     string          `json:"summary"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Priority    string          `json:"priority"`
	Type        string          `json:"type"`
	Assignee    string          `json:"assignee"`
	Reporter    string          `json:"reporter"`
	Labels      []string        `json:"labels"`
	URL         string          `json:"url"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Parent      *JiraChildItem  `json:"parent,omitempty"`
	Children    []JiraChildItem `json:"children,omitempty"`
}

type JiraComment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type JiraChildItem struct {
	Key     string `json:"key"`
	Summary string `json:"summary"`
	Type    string `json:"type"`
	Status  string `json:"status"`
}

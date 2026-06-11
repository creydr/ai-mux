package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/creydr/ai-mux/internal/provider"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

type acliNameField struct {
	Name string `json:"name"`
}

type acliUserField struct {
	DisplayName string `json:"displayName"`
}

type acliSearchFields struct {
	Summary  string         `json:"summary"`
	Status   acliNameField  `json:"status"`
	Priority acliNameField  `json:"priority"`
	Type     acliNameField  `json:"issuetype"`
	Assignee *acliUserField `json:"assignee"`
}

type acliSearchResult struct {
	Key    string           `json:"key"`
	Self   string           `json:"self"`
	Fields acliSearchFields `json:"fields"`
}

type acliParentField struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string        `json:"summary"`
		Type    acliNameField `json:"issuetype"`
		Status  acliNameField `json:"status"`
	} `json:"fields"`
}

type acliViewFields struct {
	Summary     string           `json:"summary"`
	Description json.RawMessage  `json:"description"`
	Status      acliNameField    `json:"status"`
	Priority    acliNameField    `json:"priority"`
	Type        acliNameField    `json:"issuetype"`
	Assignee    *acliUserField   `json:"assignee"`
	Reporter    *acliUserField   `json:"reporter"`
	Labels      []string         `json:"labels"`
	Created     string           `json:"created"`
	Updated     string           `json:"updated"`
	Parent      *acliParentField `json:"parent"`
}

type acliViewResult struct {
	Key    string         `json:"key"`
	Self   string         `json:"self"`
	Fields acliViewFields `json:"fields"`
}

type acliComment struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
}

type acliCommentsWrapper struct {
	Comments []acliComment `json:"comments"`
}

func (c *Client) Search(ctx context.Context, jql, orderBy string, limit int) ([]provider.JiraItem, error) {
	fullJQL := jql
	if orderBy != "" {
		fullJQL += " ORDER BY " + orderBy
	}

	args := []string{"jira", "workitem", "search",
		"--jql", fullJQL,
		"--json",
		"--fields", "key,summary,assignee,priority,status,issuetype",
	}
	if limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", limit))
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("searching jira items: %w", err)
	}

	var results []acliSearchResult
	if err := json.Unmarshal(out, &results); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}

	items := make([]provider.JiraItem, 0, len(results))
	for _, r := range results {
		if r.Key == "" {
			continue
		}
		item := provider.JiraItem{
			Key:      r.Key,
			Summary:  r.Fields.Summary,
			Status:   r.Fields.Status.Name,
			Priority: r.Fields.Priority.Name,
			Type:     r.Fields.Type.Name,
			URL:      selfToURL(r.Self, r.Key),
		}
		if r.Fields.Assignee != nil {
			item.Assignee = r.Fields.Assignee.DisplayName
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *Client) GetItem(ctx context.Context, key string) (*provider.JiraItem, error) {
	out, err := c.run(ctx, "jira", "workitem", "view", key,
		"--json",
		"--fields", "*all",
	)
	if err != nil {
		return nil, fmt.Errorf("getting jira item %s: %w", key, err)
	}

	var r acliViewResult
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("parsing item %s: %w", key, err)
	}

	item := &provider.JiraItem{
		Key:       r.Key,
		Summary:   r.Fields.Summary,
		Status:    r.Fields.Status.Name,
		Priority:  r.Fields.Priority.Name,
		Type:      r.Fields.Type.Name,
		Labels:    r.Fields.Labels,
		URL:       selfToURL(r.Self, r.Key),
		CreatedAt: parseTime(r.Fields.Created),
		UpdatedAt: parseTime(r.Fields.Updated),
	}
	item.Description = parseDescription(r.Fields.Description)
	if r.Fields.Assignee != nil {
		item.Assignee = r.Fields.Assignee.DisplayName
	}
	if r.Fields.Reporter != nil {
		item.Reporter = r.Fields.Reporter.DisplayName
	}
	if r.Fields.Parent != nil && r.Fields.Parent.Key != "" {
		item.Parent = &provider.JiraChildItem{
			Key:     r.Fields.Parent.Key,
			Summary: r.Fields.Parent.Fields.Summary,
			Type:    r.Fields.Parent.Fields.Type.Name,
			Status:  r.Fields.Parent.Fields.Status.Name,
		}
	}

	children, err := c.Search(ctx, "parent = "+key, "", 50)
	if err == nil {
		for _, child := range children {
			item.Children = append(item.Children, provider.JiraChildItem{
				Key:     child.Key,
				Summary: child.Summary,
				Type:    child.Type,
				Status:  child.Status,
			})
		}
	}

	return item, nil
}

func (c *Client) GetComments(ctx context.Context, key string) ([]provider.JiraComment, error) {
	out, err := c.run(ctx, "jira", "workitem", "comment", "list",
		"--key", key,
		"--json",
	)
	if err != nil {
		return nil, fmt.Errorf("getting comments for %s: %w", key, err)
	}

	var wrapper acliCommentsWrapper
	if err := json.Unmarshal(out, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing comments for %s: %w", key, err)
	}

	comments := make([]provider.JiraComment, len(wrapper.Comments))
	for i, r := range wrapper.Comments {
		comments[i] = provider.JiraComment{
			ID:        r.ID,
			Author:    r.Author,
			Body:      r.Body,
			CreatedAt: parseTime(r.Created),
		}
	}
	return comments, nil
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "acli", args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

func parseDescription(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var doc adfDocument
	if json.Unmarshal(raw, &doc) == nil {
		return extractADFText(&doc)
	}
	return string(raw)
}

type adfDocument struct {
	Content []adfNode `json:"content"`
}

type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
}

func extractADFText(doc *adfDocument) string {
	var b strings.Builder
	for i, node := range doc.Content {
		if i > 0 {
			b.WriteString("\n")
		}
		extractNodeText(&b, &node)
	}
	return b.String()
}

func extractNodeText(b *strings.Builder, node *adfNode) {
	if node.Text != "" {
		b.WriteString(node.Text)
	}
	for _, child := range node.Content {
		extractNodeText(b, &child)
	}
}

func selfToURL(self, key string) string {
	if self == "" {
		return ""
	}
	// self is typically like "https://jira.example.com/rest/api/2/issue/12345"
	// We want "https://jira.example.com/browse/KEY"
	idx := strings.Index(self, "/rest/")
	if idx < 0 {
		return ""
	}
	return self[:idx] + "/browse/" + key
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		time.RFC3339,
		"2006-01-02T15:04:05.000+0000",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

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

// acliSearchResult represents the JSON output from acli jira workitem search.
type acliSearchResult struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Type     string `json:"issuetype"`
	Assignee string `json:"assignee"`
	Self     string `json:"self"`
}

// acliViewResult represents the JSON output from acli jira workitem view.
type acliViewResult struct {
	Key         string `json:"key"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	Type        string `json:"issuetype"`
	Assignee    string `json:"assignee"`
	Reporter    string `json:"reporter"`
	Labels      string `json:"labels"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
	Self        string `json:"self"`
}

// acliComment represents a comment from acli jira workitem comment list.
type acliComment struct {
	ID      string `json:"id"`
	Author  string `json:"author"`
	Body    string `json:"body"`
	Created string `json:"created"`
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

	items := make([]provider.JiraItem, len(results))
	for i, r := range results {
		items[i] = provider.JiraItem{
			Key:      r.Key,
			Summary:  r.Summary,
			Status:   r.Status,
			Priority: r.Priority,
			Type:     r.Type,
			Assignee: r.Assignee,
			URL:      selfToURL(r.Self, r.Key),
		}
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

	var labels []string
	if r.Labels != "" {
		labels = strings.Split(r.Labels, ",")
		for i := range labels {
			labels[i] = strings.TrimSpace(labels[i])
		}
	}

	item := &provider.JiraItem{
		Key:         r.Key,
		Summary:     r.Summary,
		Description: r.Description,
		Status:      r.Status,
		Priority:    r.Priority,
		Type:        r.Type,
		Assignee:    r.Assignee,
		Reporter:    r.Reporter,
		Labels:      labels,
		URL:         selfToURL(r.Self, r.Key),
		CreatedAt:   parseTime(r.Created),
		UpdatedAt:   parseTime(r.Updated),
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

	var results []acliComment
	if err := json.Unmarshal(out, &results); err != nil {
		return nil, fmt.Errorf("parsing comments for %s: %w", key, err)
	}

	comments := make([]provider.JiraComment, len(results))
	for i, r := range results {
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

package github

import (
	"testing"
	"time"

	gh "github.com/google/go-github/v71/github"

	"github.com/creydr/ai-mux/internal/provider"
)

func TestMapIssue(t *testing.T) {
	now := time.Now()
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	issue := &gh.Issue{
		Number:    gh.Ptr(42),
		Title:     gh.Ptr("Fix bug"),
		Body:      gh.Ptr("Description"),
		State:     gh.Ptr("open"),
		HTMLURL:   gh.Ptr("https://github.com/owner/repo/issues/42"),
		User:      &gh.User{Login: gh.Ptr("author")},
		CreatedAt: &gh.Timestamp{Time: now},
		UpdatedAt: &gh.Timestamp{Time: now},
		Labels: []*gh.Label{
			{Name: gh.Ptr("bug")},
			{Name: gh.Ptr("urgent")},
		},
	}

	item := mapIssue(repo, issue)

	if item.Type != provider.ItemTypeIssue {
		t.Errorf("expected issue type, got %s", item.Type)
	}
	if item.Number != 42 {
		t.Errorf("expected number 42, got %d", item.Number)
	}
	if item.Title != "Fix bug" {
		t.Errorf("expected title Fix bug, got %s", item.Title)
	}
	if item.Author != "author" {
		t.Errorf("expected author, got %s", item.Author)
	}
	if len(item.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(item.Labels))
	}
	if item.ID != "owner/repo/issues/42" {
		t.Errorf("expected owner/repo/issues/42, got %s", item.ID)
	}
}

func TestMapIssue_NilFields(t *testing.T) {
	repo := provider.RepoRef{Owner: "o", Repo: "r"}
	issue := &gh.Issue{
		Number: gh.Ptr(1),
	}

	item := mapIssue(repo, issue)
	if item.Number != 1 {
		t.Errorf("expected number 1, got %d", item.Number)
	}
	if item.Author != "" {
		t.Errorf("expected empty author, got %s", item.Author)
	}
}

func TestMapPR(t *testing.T) {
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}
	mergeable := true

	pr := &gh.PullRequest{
		Number:    gh.Ptr(10),
		Title:     gh.Ptr("Add feature"),
		Body:      gh.Ptr("PR body"),
		State:     gh.Ptr("open"),
		Draft:     gh.Ptr(true),
		Mergeable: &mergeable,
		HTMLURL:   gh.Ptr("https://github.com/owner/repo/pull/10"),
		User:      &gh.User{Login: gh.Ptr("dev")},
		CreatedAt: &gh.Timestamp{Time: time.Now()},
		Labels:    []*gh.Label{{Name: gh.Ptr("enhancement")}},
	}

	item := mapPR(repo, pr)

	if item.Type != provider.ItemTypePR {
		t.Errorf("expected PR type, got %s", item.Type)
	}
	if !item.Draft {
		t.Error("expected draft PR")
	}
	if item.Mergeable == nil || !*item.Mergeable {
		t.Error("expected mergeable true")
	}
	if item.ID != "owner/repo/pulls/10" {
		t.Errorf("expected owner/repo/pulls/10, got %s", item.ID)
	}
}

func TestMapReview(t *testing.T) {
	review := &gh.PullRequestReview{
		ID:          gh.Ptr(int64(1)),
		State:       gh.Ptr("APPROVED"),
		Body:        gh.Ptr("LGTM"),
		User:        &gh.User{Login: gh.Ptr("reviewer")},
		SubmittedAt: &gh.Timestamp{Time: time.Now()},
	}

	r := mapReview(review)

	if r.State != "APPROVED" {
		t.Errorf("expected APPROVED, got %s", r.State)
	}
	if r.Author != "reviewer" {
		t.Errorf("expected reviewer, got %s", r.Author)
	}
}

func TestMapComment(t *testing.T) {
	comment := &gh.IssueComment{
		ID:        gh.Ptr(int64(100)),
		Body:      gh.Ptr("Nice work"),
		User:      &gh.User{Login: gh.Ptr("commenter")},
		CreatedAt: &gh.Timestamp{Time: time.Now()},
	}

	c := mapComment(comment)

	if c.Body != "Nice work" {
		t.Errorf("expected Nice work, got %s", c.Body)
	}
	if c.Author != "commenter" {
		t.Errorf("expected commenter, got %s", c.Author)
	}
}

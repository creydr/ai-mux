package github

import (
	"fmt"
	"time"

	gh "github.com/google/go-github/v71/github"

	"github.com/creydr/ai-mux/internal/provider"
)

func mapIssue(repo provider.RepoRef, issue *gh.Issue) provider.Item {
	item := provider.Item{
		ID:     fmt.Sprintf("%s/issues/%d", repo.String(), issue.GetNumber()),
		Type:   provider.ItemTypeIssue,
		Repo:   repo,
		Number: issue.GetNumber(),
		Title:  issue.GetTitle(),
		Body:   issue.GetBody(),
		State:  issue.GetState(),
		URL:    issue.GetHTMLURL(),
	}

	if issue.User != nil {
		item.Author = issue.User.GetLogin()
	}

	if issue.CreatedAt != nil {
		item.CreatedAt = issue.CreatedAt.Time
	}
	if issue.UpdatedAt != nil {
		item.UpdatedAt = issue.UpdatedAt.Time
	}

	for _, label := range issue.Labels {
		item.Labels = append(item.Labels, label.GetName())
	}

	return item
}

func mapPR(repo provider.RepoRef, pr *gh.PullRequest) provider.Item {
	item := provider.Item{
		ID:     fmt.Sprintf("%s/pulls/%d", repo.String(), pr.GetNumber()),
		Type:   provider.ItemTypePR,
		Repo:   repo,
		Number: pr.GetNumber(),
		Title:  pr.GetTitle(),
		Body:   pr.GetBody(),
		State:  pr.GetState(),
		URL:    pr.GetHTMLURL(),
		Draft:  pr.GetDraft(),
	}

	if pr.User != nil {
		item.Author = pr.User.GetLogin()
	}

	if pr.CreatedAt != nil {
		item.CreatedAt = pr.CreatedAt.Time
	}
	if pr.UpdatedAt != nil {
		item.UpdatedAt = pr.UpdatedAt.Time
	}

	if pr.Mergeable != nil {
		m := pr.GetMergeable()
		item.Mergeable = &m
	}

	for _, label := range pr.Labels {
		item.Labels = append(item.Labels, label.GetName())
	}

	return item
}

func mapReview(review *gh.PullRequestReview) provider.Review {
	r := provider.Review{
		ID:    fmt.Sprintf("%d", review.GetID()),
		State: review.GetState(),
		Body:  review.GetBody(),
	}

	if review.User != nil {
		r.Author = review.User.GetLogin()
	}

	if review.SubmittedAt != nil {
		r.SubmittedAt = review.SubmittedAt.Time
	}

	return r
}

func mapComment(comment *gh.IssueComment) provider.Comment {
	c := provider.Comment{
		ID:   fmt.Sprintf("%d", comment.GetID()),
		Body: comment.GetBody(),
	}

	if comment.User != nil {
		c.Author = comment.User.GetLogin()
	}

	if comment.CreatedAt != nil {
		c.CreatedAt = comment.CreatedAt.Time
	}

	return c
}

func timePtr(t time.Time) *gh.Timestamp {
	return &gh.Timestamp{Time: t}
}

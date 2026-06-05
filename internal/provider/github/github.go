package github

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	gh "github.com/google/go-github/v71/github"

	"github.com/creydr/ai-mux/internal/provider"
)

type GitHubProvider struct {
	client *gh.Client
}

func New(token string) *GitHubProvider {
	client := gh.NewClient(nil).WithAuthToken(token)
	return &GitHubProvider{client: client}
}

func NewFromGHCLI() (*GitHubProvider, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return nil, fmt.Errorf("getting token from gh CLI: %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return nil, fmt.Errorf("gh CLI returned empty token")
	}
	return &GitHubProvider{client: gh.NewClient(nil).WithAuthToken(token)}, nil
}

func NewWithClient(client *gh.Client) *GitHubProvider {
	return &GitHubProvider{client: client}
}

func (p *GitHubProvider) Name() string {
	return "github"
}

func (p *GitHubProvider) ListIssues(ctx context.Context, repo provider.RepoRef, opts provider.ListOptions) ([]provider.Item, error) {
	ghOpts := &gh.IssueListByRepoOptions{
		State:     opts.State,
		Sort:      "updated",
		Direction: "desc",
		ListOptions: gh.ListOptions{
			PerPage: perPage(opts.PerPage),
		},
	}

	if !opts.Since.IsZero() {
		ghOpts.Since = opts.Since
	}
	if ghOpts.State == "" {
		ghOpts.State = "open"
	}

	var items []provider.Item
	for {
		issues, resp, err := p.client.Issues.ListByRepo(ctx, repo.Owner, repo.Repo, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("listing issues: %w", err)
		}

		for _, issue := range issues {
			if issue.PullRequestLinks != nil {
				continue
			}
			items = append(items, mapIssue(repo, issue))
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	return items, nil
}

func (p *GitHubProvider) ListPRs(ctx context.Context, repo provider.RepoRef, opts provider.ListOptions) ([]provider.Item, error) {
	ghOpts := &gh.PullRequestListOptions{
		State:     opts.State,
		Sort:      "updated",
		Direction: "desc",
		ListOptions: gh.ListOptions{
			PerPage: perPage(opts.PerPage),
		},
	}
	if ghOpts.State == "" {
		ghOpts.State = "open"
	}

	var items []provider.Item
	for {
		prs, resp, err := p.client.PullRequests.List(ctx, repo.Owner, repo.Repo, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("listing PRs: %w", err)
		}

		for _, pr := range prs {
			items = append(items, mapPR(repo, pr))
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	return items, nil
}

func (p *GitHubProvider) GetItem(ctx context.Context, repo provider.RepoRef, itemType provider.ItemType, number int) (*provider.Item, error) {
	switch itemType {
	case provider.ItemTypeIssue:
		issue, _, err := p.client.Issues.Get(ctx, repo.Owner, repo.Repo, number)
		if err != nil {
			return nil, fmt.Errorf("getting issue: %w", err)
		}
		item := mapIssue(repo, issue)
		return &item, nil

	case provider.ItemTypePR:
		pr, _, err := p.client.PullRequests.Get(ctx, repo.Owner, repo.Repo, number)
		if err != nil {
			return nil, fmt.Errorf("getting PR: %w", err)
		}
		item := mapPR(repo, pr)
		return &item, nil

	default:
		return nil, fmt.Errorf("unknown item type: %s", itemType)
	}
}

func (p *GitHubProvider) GetDiff(ctx context.Context, repo provider.RepoRef, number int) (string, error) {
	diff, _, err := p.client.PullRequests.GetRaw(ctx, repo.Owner, repo.Repo, number, gh.RawOptions{Type: gh.Diff})
	if err != nil {
		return "", fmt.Errorf("getting diff: %w", err)
	}
	return diff, nil
}

func (p *GitHubProvider) ListReviews(ctx context.Context, repo provider.RepoRef, number int) ([]provider.Review, error) {
	ghOpts := &gh.ListOptions{PerPage: 100}

	var reviews []provider.Review
	for {
		ghReviews, resp, err := p.client.PullRequests.ListReviews(ctx, repo.Owner, repo.Repo, number, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("listing reviews: %w", err)
		}

		for _, review := range ghReviews {
			reviews = append(reviews, mapReview(review))
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	return reviews, nil
}

func (p *GitHubProvider) ListComments(ctx context.Context, repo provider.RepoRef, number int) ([]provider.Comment, error) {
	ghOpts := &gh.IssueListCommentsOptions{
		Sort:      gh.Ptr("updated"),
		Direction: gh.Ptr("desc"),
		ListOptions: gh.ListOptions{
			PerPage: 100,
		},
	}

	var comments []provider.Comment
	for {
		ghComments, resp, err := p.client.Issues.ListComments(ctx, repo.Owner, repo.Repo, number, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("listing comments: %w", err)
		}

		for _, comment := range ghComments {
			comments = append(comments, mapComment(comment))
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	return comments, nil
}

func (p *GitHubProvider) AssignUser(ctx context.Context, repo provider.RepoRef, number int, username string) error {
	_, _, err := p.client.Issues.AddAssignees(ctx, repo.Owner, repo.Repo, number, []string{username})
	if err != nil {
		return fmt.Errorf("assigning user: %w", err)
	}
	return nil
}

func perPage(n int) int {
	if n <= 0 {
		return 30
	}
	return n
}

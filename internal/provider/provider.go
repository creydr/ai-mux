package provider

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type ItemType string

const (
	ItemTypeIssue ItemType = "issue"
	ItemTypePR    ItemType = "pr"
)

type RepoRef struct {
	Owner string
	Repo  string
}

func (r RepoRef) String() string {
	return r.Owner + "/" + r.Repo
}

func ParseRepoRef(s string) (RepoRef, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return RepoRef{}, fmt.Errorf("invalid repo reference %q: must be owner/repo", s)
	}
	return RepoRef{Owner: parts[0], Repo: parts[1]}, nil
}

type Item struct {
	ID        string
	Type      ItemType
	Repo      RepoRef
	Number    int
	Title     string
	Body      string
	Author    string
	Labels    []string
	State     string
	URL       string
	CreatedAt time.Time
	UpdatedAt time.Time
	Draft     bool
	Mergeable *bool
}

type Review struct {
	ID          string
	Author      string
	State       string
	Body        string
	SubmittedAt time.Time
}

type Comment struct {
	ID        string
	Author    string
	Body      string
	CreatedAt time.Time
}

type ListOptions struct {
	State   string
	Since   time.Time
	PerPage int
	Limit   int
}

type Provider interface {
	Name() string
	ListIssues(ctx context.Context, repo RepoRef, opts ListOptions) ([]Item, error)
	ListPRs(ctx context.Context, repo RepoRef, opts ListOptions) ([]Item, error)
	GetItem(ctx context.Context, repo RepoRef, itemType ItemType, number int) (*Item, error)
	GetDiff(ctx context.Context, repo RepoRef, number int) (string, error)
	ListReviews(ctx context.Context, repo RepoRef, number int) ([]Review, error)
	ListComments(ctx context.Context, repo RepoRef, number int) ([]Comment, error)
}

type Assigner interface {
	AssignUser(ctx context.Context, repo RepoRef, number int, username string) error
}

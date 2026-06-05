package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v71/github"

	"github.com/creydr/ai-mux/internal/provider"
)

func TestListIssues(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		issues := []*gh.Issue{
			{
				Number:  gh.Ptr(1),
				Title:   gh.Ptr("Bug report"),
				State:   gh.Ptr("open"),
				HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/1"),
				User:    &gh.User{Login: gh.Ptr("user1")},
			},
			{
				Number:           gh.Ptr(2),
				Title:            gh.Ptr("PR disguised as issue"),
				PullRequestLinks: &gh.PullRequestLinks{URL: gh.Ptr("https://api.github.com/repos/owner/repo/pulls/2")},
			},
			{
				Number:  gh.Ptr(3),
				Title:   gh.Ptr("Feature request"),
				State:   gh.Ptr("open"),
				HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/3"),
				User:    &gh.User{Login: gh.Ptr("user2")},
			},
		}
		json.NewEncoder(w).Encode(issues)
	})

	p := newTestProvider(t, mux)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	items, err := p.ListIssues(context.Background(), repo, provider.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 issues (PR filtered out), got %d", len(items))
	}
	if items[0].Title != "Bug report" {
		t.Errorf("expected Bug report, got %s", items[0].Title)
	}
	if items[1].Title != "Feature request" {
		t.Errorf("expected Feature request, got %s", items[1].Title)
	}
}

func TestListPRs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		prs := []*gh.PullRequest{
			{
				Number:  gh.Ptr(10),
				Title:   gh.Ptr("Add feature"),
				State:   gh.Ptr("open"),
				Draft:   gh.Ptr(false),
				HTMLURL: gh.Ptr("https://github.com/owner/repo/pull/10"),
				User:    &gh.User{Login: gh.Ptr("dev")},
			},
		}
		json.NewEncoder(w).Encode(prs)
	})

	p := newTestProvider(t, mux)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	items, err := p.ListPRs(context.Background(), repo, provider.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(items))
	}
	if items[0].Type != provider.ItemTypePR {
		t.Errorf("expected PR type, got %s", items[0].Type)
	}
	if items[0].Title != "Add feature" {
		t.Errorf("expected Add feature, got %s", items[0].Title)
	}
}

func TestGetDiff(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/pulls/10", func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if accept == "application/vnd.github.v3.diff" {
			w.Write([]byte("diff --git a/file.go b/file.go\n+new line"))
			return
		}
		pr := &gh.PullRequest{Number: gh.Ptr(10)}
		json.NewEncoder(w).Encode(pr)
	})

	p := newTestProvider(t, mux)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	diff, err := p.GetDiff(context.Background(), repo, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if diff == "" {
		t.Error("expected non-empty diff")
	}
}

func TestListReviews(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/pulls/10/reviews", func(w http.ResponseWriter, r *http.Request) {
		reviews := []*gh.PullRequestReview{
			{
				ID:    gh.Ptr(int64(1)),
				State: gh.Ptr("APPROVED"),
				Body:  gh.Ptr("LGTM"),
				User:  &gh.User{Login: gh.Ptr("reviewer")},
			},
		}
		json.NewEncoder(w).Encode(reviews)
	})

	p := newTestProvider(t, mux)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	reviews, err := p.ListReviews(context.Background(), repo, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reviews) != 1 {
		t.Fatalf("expected 1 review, got %d", len(reviews))
	}
	if reviews[0].State != "APPROVED" {
		t.Errorf("expected APPROVED, got %s", reviews[0].State)
	}
}

func TestListComments(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/issues/42/comments", func(w http.ResponseWriter, r *http.Request) {
		comments := []*gh.IssueComment{
			{
				ID:   gh.Ptr(int64(100)),
				Body: gh.Ptr("Nice work"),
				User: &gh.User{Login: gh.Ptr("commenter")},
			},
		}
		json.NewEncoder(w).Encode(comments)
	})

	p := newTestProvider(t, mux)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	comments, err := p.ListComments(context.Background(), repo, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Body != "Nice work" {
		t.Errorf("expected Nice work, got %s", comments[0].Body)
	}
}

func TestGetItem_Issue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/issues/42", func(w http.ResponseWriter, r *http.Request) {
		issue := &gh.Issue{
			Number:  gh.Ptr(42),
			Title:   gh.Ptr("Test issue"),
			State:   gh.Ptr("open"),
			HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/42"),
		}
		json.NewEncoder(w).Encode(issue)
	})

	p := newTestProvider(t, mux)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	item, err := p.GetItem(context.Background(), repo, provider.ItemTypeIssue, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.Type != provider.ItemTypeIssue {
		t.Errorf("expected issue type, got %s", item.Type)
	}
	if item.Number != 42 {
		t.Errorf("expected number 42, got %d", item.Number)
	}
}

func TestGetItem_PR(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/pulls/10", func(w http.ResponseWriter, r *http.Request) {
		pr := &gh.PullRequest{
			Number:  gh.Ptr(10),
			Title:   gh.Ptr("Test PR"),
			State:   gh.Ptr("open"),
			HTMLURL: gh.Ptr("https://github.com/owner/repo/pull/10"),
		}
		json.NewEncoder(w).Encode(pr)
	})

	p := newTestProvider(t, mux)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	item, err := p.GetItem(context.Background(), repo, provider.ItemTypePR, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.Type != provider.ItemTypePR {
		t.Errorf("expected PR type, got %s", item.Type)
	}
}

func newTestProvider(t *testing.T, mux *http.ServeMux) *GitHubProvider {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := gh.NewClient(nil)
	baseURL, _ := client.BaseURL.Parse(server.URL + "/")
	client.BaseURL = baseURL

	return NewWithClient(client)
}

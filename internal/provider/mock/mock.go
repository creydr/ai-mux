package mock

import (
	"context"
	"sync"

	"github.com/creydr/ai-mux/internal/provider"
)

type Provider struct {
	mu       sync.Mutex
	issues   map[string][]provider.Item
	prs      map[string][]provider.Item
	items    map[string]*provider.Item
	diffs    map[int]string
	reviews  map[int][]provider.Review
	comments map[int][]provider.Comment
	err      error
}

func New() *Provider {
	return &Provider{
		issues:   make(map[string][]provider.Item),
		prs:      make(map[string][]provider.Item),
		items:    make(map[string]*provider.Item),
		diffs:    make(map[int]string),
		reviews:  make(map[int][]provider.Review),
		comments: make(map[int][]provider.Comment),
	}
}

func (p *Provider) Name() string { return "mock" }

func (p *Provider) SetError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

func (p *Provider) AddIssues(repo provider.RepoRef, items ...provider.Item) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.issues[repo.String()] = append(p.issues[repo.String()], items...)
}

func (p *Provider) AddPRs(repo provider.RepoRef, items ...provider.Item) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prs[repo.String()] = append(p.prs[repo.String()], items...)
}

func (p *Provider) SetDiff(number int, diff string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.diffs[number] = diff
}

func (p *Provider) SetReviews(number int, reviews []provider.Review) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reviews[number] = reviews
}

func (p *Provider) SetComments(number int, comments []provider.Comment) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.comments[number] = comments
}

func (p *Provider) ListIssues(_ context.Context, repo provider.RepoRef, _ provider.ListOptions) ([]provider.Item, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	return p.issues[repo.String()], nil
}

func (p *Provider) ListPRs(_ context.Context, repo provider.RepoRef, _ provider.ListOptions) ([]provider.Item, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	return p.prs[repo.String()], nil
}

func (p *Provider) GetItem(_ context.Context, repo provider.RepoRef, itemType provider.ItemType, number int) (*provider.Item, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}

	var items []provider.Item
	if itemType == provider.ItemTypeIssue {
		items = p.issues[repo.String()]
	} else {
		items = p.prs[repo.String()]
	}

	for _, item := range items {
		if item.Number == number {
			return &item, nil
		}
	}
	return nil, nil
}

func (p *Provider) GetDiff(_ context.Context, _ provider.RepoRef, number int) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return "", p.err
	}
	return p.diffs[number], nil
}

func (p *Provider) ListReviews(_ context.Context, _ provider.RepoRef, number int) ([]provider.Review, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	return p.reviews[number], nil
}

func (p *Provider) ListComments(_ context.Context, _ provider.RepoRef, number int) ([]provider.Comment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	return p.comments[number], nil
}

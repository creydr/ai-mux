package poller

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/store"
)

type Poller struct {
	provider provider.Provider
	store    store.Store
	bus      *event.Bus
	repos    []provider.RepoRef
	interval time.Duration
}

func New(prov provider.Provider, st store.Store, bus *event.Bus, repos []provider.RepoRef, interval time.Duration) *Poller {
	return &Poller{
		provider: prov,
		store:    st,
		bus:      bus,
		repos:    repos,
		interval: interval,
	}
}

func (p *Poller) Start(ctx context.Context) error {
	if err := p.PollOnce(ctx); err != nil {
		log.Printf("initial poll error: %v", err)
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.PollOnce(ctx); err != nil {
				log.Printf("poll error: %v", err)
			}
		}
	}
}

func (p *Poller) PollOnce(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, repo := range p.repos {
		wg.Add(1)
		go func(r provider.RepoRef) {
			defer wg.Done()
			if err := p.pollRepo(ctx, r); err != nil {
				log.Printf("error polling %s: %v", r, err)
			}
		}(repo)
	}
	wg.Wait()
	return nil
}

func (p *Poller) pollRepo(ctx context.Context, repo provider.RepoRef) error {
	since, err := p.store.GetLastPollTime(repo)
	if err != nil {
		return err
	}

	opts := provider.ListOptions{
		State: "open",
		Since: since,
	}

	issues, err := p.provider.ListIssues(ctx, repo, opts)
	if err != nil {
		return err
	}
	p.processItems(issues, event.TypeNewIssue, event.TypeIssueUpdated)

	prs, err := p.provider.ListPRs(ctx, repo, opts)
	if err != nil {
		return err
	}
	p.processItems(prs, event.TypeNewPR, event.TypePRUpdated)

	if err := p.store.SetLastPollTime(repo, time.Now()); err != nil {
		return err
	}

	return nil
}

func (p *Poller) processItems(items []provider.Item, newType, updatedType event.Type) {
	for _, item := range items {
		existing, err := p.store.GetItemState(item.ID)
		if err != nil {
			log.Printf("error getting item state for %s: %v", item.ID, err)
			continue
		}

		itemCopy := item
		if existing == nil {
			p.bus.Publish(event.Event{
				Type:      newType,
				Item:      &itemCopy,
				Timestamp: time.Now(),
			})
			if err := p.store.SetItemState(store.ItemState{
				ItemID:     item.ID,
				Read:       false,
				LastSeenAt: item.UpdatedAt,
			}); err != nil {
				log.Printf("error saving item state for %s: %v", item.ID, err)
			}
		} else if item.UpdatedAt.After(existing.LastSeenAt) {
			p.bus.Publish(event.Event{
				Type:      updatedType,
				Item:      &itemCopy,
				Timestamp: time.Now(),
			})
			existing.LastSeenAt = item.UpdatedAt
			if err := p.store.SetItemState(*existing); err != nil {
				log.Printf("error updating item state for %s: %v", item.ID, err)
			}
		}
	}
}

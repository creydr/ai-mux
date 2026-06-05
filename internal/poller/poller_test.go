package poller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/provider/mock"
	"github.com/creydr/ai-mux/internal/store"
	"github.com/creydr/ai-mux/internal/store/jsonfile"
)

func TestPoller_PollOnce_NewIssues(t *testing.T) {
	prov, st, bus := setup(t)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	prov.AddIssues(repo,
		provider.Item{ID: "owner/repo/issues/1", Number: 1, Title: "Issue 1", Type: provider.ItemTypeIssue, UpdatedAt: time.Now()},
		provider.Item{ID: "owner/repo/issues/2", Number: 2, Title: "Issue 2", Type: provider.ItemTypeIssue, UpdatedAt: time.Now()},
	)

	ch := bus.Subscribe(event.TypeNewIssue)
	p := New(prov, st, bus, []provider.RepoRef{repo}, time.Minute)

	if err := p.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	events := drain(ch, 2, time.Second)
	if len(events) != 2 {
		t.Fatalf("expected 2 new issue events, got %d", len(events))
	}
}

func TestPoller_PollOnce_NoNewItems(t *testing.T) {
	prov, st, bus := setup(t)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	now := time.Now()
	prov.AddIssues(repo,
		provider.Item{ID: "owner/repo/issues/1", Number: 1, UpdatedAt: now},
	)

	st.SetItemState(store.ItemState{ItemID: "owner/repo/issues/1", LastSeenAt: now})

	ch := bus.Subscribe()
	p := New(prov, st, bus, []provider.RepoRef{repo}, time.Minute)

	p.PollOnce(context.Background())

	events := drain(ch, 0, 100*time.Millisecond)
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestPoller_PollOnce_UpdatedItem(t *testing.T) {
	prov, st, bus := setup(t)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	past := time.Now().Add(-time.Hour)
	now := time.Now()

	st.SetItemState(store.ItemState{ItemID: "owner/repo/issues/1", LastSeenAt: past})
	prov.AddIssues(repo,
		provider.Item{ID: "owner/repo/issues/1", Number: 1, UpdatedAt: now},
	)

	ch := bus.Subscribe(event.TypeIssueUpdated)
	p := New(prov, st, bus, []provider.RepoRef{repo}, time.Minute)

	p.PollOnce(context.Background())

	events := drain(ch, 1, time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 updated event, got %d", len(events))
	}
}

func TestPoller_PollOnce_MultipleRepos(t *testing.T) {
	prov, st, bus := setup(t)
	repo1 := provider.RepoRef{Owner: "owner", Repo: "repo1"}
	repo2 := provider.RepoRef{Owner: "owner", Repo: "repo2"}

	prov.AddIssues(repo1, provider.Item{ID: "owner/repo1/issues/1", Number: 1, UpdatedAt: time.Now()})
	prov.AddIssues(repo2, provider.Item{ID: "owner/repo2/issues/1", Number: 1, UpdatedAt: time.Now()})

	ch := bus.Subscribe(event.TypeNewIssue)
	p := New(prov, st, bus, []provider.RepoRef{repo1, repo2}, time.Minute)

	p.PollOnce(context.Background())

	events := drain(ch, 2, time.Second)
	if len(events) != 2 {
		t.Fatalf("expected 2 events from 2 repos, got %d", len(events))
	}
}

func TestPoller_Start_Cancellation(t *testing.T) {
	prov, st, bus := setup(t)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	p := New(prov, st, bus, []provider.RepoRef{repo}, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := p.Start(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected deadline exceeded, got %v", err)
	}
}

func TestPoller_PollOnce_ProviderError(t *testing.T) {
	prov, st, bus := setup(t)
	repo := provider.RepoRef{Owner: "owner", Repo: "repo"}

	prov.SetError(fmt.Errorf("API error"))

	p := New(prov, st, bus, []provider.RepoRef{repo}, time.Minute)

	err := p.PollOnce(context.Background())
	if err != nil {
		t.Fatalf("PollOnce should not return error (logs internally), got %v", err)
	}
}

func setup(t *testing.T) (*mock.Provider, *jsonfile.Store, *event.Bus) {
	t.Helper()
	prov := mock.New()
	st, err := jsonfile.New(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatal(err)
	}
	bus := event.NewBus()
	t.Cleanup(bus.Close)
	return prov, st, bus
}

func drain(ch <-chan event.Event, expected int, timeout time.Duration) []event.Event {
	var events []event.Event
	timer := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			events = append(events, ev)
			if len(events) >= expected && expected > 0 {
				return events
			}
		case <-timer:
			return events
		}
	}
}

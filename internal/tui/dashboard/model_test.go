package dashboard

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/tui/attach"
)

func testItems() ([]provider.Item, []provider.Item) {
	issues := []provider.Item{
		{ID: "o/r/issues/1", Number: 1, Title: "Bug report", Type: provider.ItemTypeIssue, Repo: provider.RepoRef{Owner: "o", Repo: "r"}, URL: "https://github.com/o/r/issues/1"},
		{ID: "o/r/issues/2", Number: 2, Title: "Feature request", Type: provider.ItemTypeIssue, Repo: provider.RepoRef{Owner: "o", Repo: "r"}},
	}
	prs := []provider.Item{
		{ID: "o/r/prs/10", Number: 10, Title: "Fix typo", Type: provider.ItemTypePR, Repo: provider.RepoRef{Owner: "o", Repo: "r"}, URL: "https://github.com/o/r/pull/10"},
	}
	return issues, prs
}

func multiRepoItems(perRepo int) []provider.Item {
	var items []provider.Item
	repos := []provider.RepoRef{
		{Owner: "a", Repo: "alpha"},
		{Owner: "b", Repo: "beta"},
	}
	for _, repo := range repos {
		for i := range perRepo {
			items = append(items, provider.Item{
				ID:     fmt.Sprintf("%s/%s/issues/%d", repo.Owner, repo.Repo, i+1),
				Number: i + 1,
				Title:  fmt.Sprintf("Issue %d in %s", i+1, repo.Repo),
				Type:   provider.ItemTypeIssue,
				Repo:   repo,
			})
		}
	}
	return items
}

func TestModel_InitWithoutConn(t *testing.T) {
	m := New(nil, 3, nil, "")
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil without a connection")
	}
	if m.loading {
		t.Error("loading should be false without a connection")
	}
}

func TestModel_LoadingState(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.loading = true

	view := m.View()
	if !containsString(view.Content, "Loading...") {
		t.Error("view should show Loading... when loading")
	}

	issues, prs := testItems()
	updated, _ := m.Update(itemsReceivedMsg{issues: issues, prs: prs})
	m = updated.(Model)

	if m.loading {
		t.Error("loading should be false after items received")
	}
	view = m.View()
	if containsString(view.Content, "Loading...") {
		t.Error("view should not show Loading... after items received")
	}
}

func TestModel_LoadingClearedOnError(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.loading = true

	updated, _ := m.Update(errMsg{err: fmt.Errorf("fail")})
	m = updated.(Model)

	if m.loading {
		t.Error("loading should be false after error")
	}
}

func TestModel_TabSwitch(t *testing.T) {
	m := New(nil, 3, nil, "")
	issues, prs := testItems()
	m.issues = issues
	m.prs = prs

	if m.activeTab != tabIssues {
		t.Fatal("should start on issues tab")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	if m.activeTab != tabPRs {
		t.Error("should switch to PRs tab")
	}
	if m.cursor != 0 {
		t.Error("cursor should reset on tab switch")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	if m.activeTab != tabSessions {
		t.Error("should switch to sessions tab")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	if m.activeTab != tabIssues {
		t.Error("should wrap back to issues tab")
	}
}

func TestModel_NavigateDown(t *testing.T) {
	m := New(nil, 3, nil, "")
	issues, _ := testItems()
	m.issues = issues

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = updated.(Model)

	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	m = updated.(Model)

	if m.cursor != 1 {
		t.Error("cursor should not go past last item")
	}
}

func TestModel_NavigateUp(t *testing.T) {
	m := New(nil, 3, nil, "")
	issues, _ := testItems()
	m.issues = issues
	m.cursor = 1

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'k'})
	m = updated.(Model)

	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	m = updated.(Model)

	if m.cursor != 0 {
		t.Error("cursor should not go below 0")
	}
}

func TestModel_NavigateWithArrowKeys(t *testing.T) {
	m := New(nil, 3, nil, "")
	issues, _ := testItems()
	m.issues = issues

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	if m.cursor != 1 {
		t.Error("down arrow should move cursor down")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = updated.(Model)
	if m.cursor != 0 {
		t.Error("up arrow should move cursor up")
	}
}

func TestModel_ItemsReceived(t *testing.T) {
	m := New(nil, 3, nil, "")
	issues, prs := testItems()

	updated, _ := m.Update(itemsReceivedMsg{issues: issues, prs: prs})
	m = updated.(Model)

	if len(m.issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(m.issues))
	}
	if len(m.prs) != 1 {
		t.Errorf("expected 1 PR, got %d", len(m.prs))
	}
	if m.cursor != 0 {
		t.Error("cursor should reset after items received")
	}
}

func TestModel_EventReceived_NewIssue(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.activeTab = tabPRs

	updated, _ := m.Update(eventReceivedMsg{event: event.Event{
		Type: event.TypeNewIssue,
		Item: &provider.Item{ID: "o/r/issues/99", Number: 99, Title: "New bug"},
	}})
	m = updated.(Model)

	if len(m.issues) != 1 {
		t.Error("new issue should be added")
	}
	if m.issueBadge != 1 {
		t.Error("badge should increment when on different tab")
	}
}

func TestModel_EventReceived_NewPR(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.activeTab = tabIssues

	updated, _ := m.Update(eventReceivedMsg{event: event.Event{
		Type: event.TypeNewPR,
		Item: &provider.Item{ID: "o/r/prs/50", Number: 50, Title: "New feature"},
	}})
	m = updated.(Model)

	if len(m.prs) != 1 {
		t.Error("new PR should be added")
	}
	if m.prBadge != 1 {
		t.Error("PR badge should increment when on different tab")
	}
}

func TestModel_EventReceived_NoBadgeOnActiveTab(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.activeTab = tabIssues

	updated, _ := m.Update(eventReceivedMsg{event: event.Event{
		Type: event.TypeNewIssue,
		Item: &provider.Item{ID: "o/r/issues/99", Number: 99, Title: "New bug"},
	}})
	m = updated.(Model)

	if m.issueBadge != 0 {
		t.Error("badge should not increment when on the active tab")
	}
}

func TestModel_BadgeClearsOnTabSwitch(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.activeTab = tabIssues
	m.prBadge = 3

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	if m.prBadge != 0 {
		t.Error("badge should clear when switching to that tab")
	}
}

func TestModel_Quit(t *testing.T) {
	m := New(nil, 3, nil, "")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Error("ctrl-c should produce a quit command")
	}
}

func TestModel_WindowResize(t *testing.T) {
	m := New(nil, 3, nil, "")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	if m.width != 120 || m.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

func TestModel_ErrorMessage(t *testing.T) {
	m := New(nil, 3, nil, "")
	updated, _ := m.Update(errMsg{err: fmt.Errorf("connection failed")})
	m = updated.(Model)

	if m.err == nil {
		t.Error("error should be set")
	}
	view := m.View()
	if !containsString(view.Content, "connection failed") {
		t.Error("error should appear in view")
	}
}

func TestModel_View_ShowsItems(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.width = 80
	issues, _ := testItems()
	m.issues = issues
	m.updateRepoList()
	m.rebuildViewport()

	view := m.View()
	if !containsString(view.Content, "Bug report") {
		t.Error("view should show issue title")
	}
}

func TestModel_SelectedItem(t *testing.T) {
	m := New(nil, 3, nil, "")
	issues, _ := testItems()
	m.issues = issues
	m.cursor = 0

	item := m.selectedItem()
	if item == nil || item.Number != 1 {
		t.Error("should select first issue")
	}
}

func TestModel_SelectedItem_Empty(t *testing.T) {
	m := New(nil, 3, nil, "")
	item := m.selectedItem()
	if item != nil {
		t.Error("should return nil for empty list")
	}
}

func TestModel_UpdatedEvent(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.issues = []provider.Item{
		{ID: "o/r/issues/1", Number: 1, Title: "Old title", UpdatedAt: time.Now()},
	}

	updated, _ := m.Update(eventReceivedMsg{event: event.Event{
		Type: event.TypeIssueUpdated,
		Item: &provider.Item{ID: "o/r/issues/1", Number: 1, Title: "New title"},
	}})
	m = updated.(Model)

	if len(m.issues) != 1 {
		t.Error("should still have 1 issue")
	}
	if m.issues[0].Title != "New title" {
		t.Errorf("expected updated title, got %q", m.issues[0].Title)
	}
}

func manyItems(n int) []provider.Item {
	items := make([]provider.Item, n)
	for i := range n {
		items[i] = provider.Item{
			ID:     fmt.Sprintf("o/r/issues/%d", i+1),
			Number: i + 1,
			Title:  fmt.Sprintf("Issue %d", i+1),
			Type:   provider.ItemTypeIssue,
			Repo:   provider.RepoRef{Owner: "o", Repo: "r"},
		}
	}
	return items
}

func TestModel_ScrollDown(t *testing.T) {
	m := New(nil, 100, nil, "")
	m.height = 16
	m.issues = manyItems(30)
	m.fullLoaded["o/r"] = true

	for range 15 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
		m = updated.(Model)
	}

	if m.cursor != 15 {
		t.Errorf("expected cursor 15, got %d", m.cursor)
	}

	view := m.View()
	row := m.visibleItems()[m.cursor]
	if row.item != nil && !containsString(view.Content, row.item.Title) {
		t.Error("cursor item should be visible in view")
	}
}

func TestModel_ScrollUp(t *testing.T) {
	m := New(nil, 100, nil, "")
	m.height = 16
	m.issues = manyItems(30)
	m.fullLoaded["o/r"] = true
	m.cursor = 20
	m.rebuildViewport()

	for range 10 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'k'})
		m = updated.(Model)
	}

	if m.cursor != 10 {
		t.Errorf("expected cursor 10, got %d", m.cursor)
	}

	view := m.View()
	row := m.visibleItems()[m.cursor]
	if row.item != nil && !containsString(view.Content, row.item.Title) {
		t.Error("cursor item should be visible in view after scrolling up")
	}
}

func TestModel_TabSwitchResetsCursor(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.issues = manyItems(30)
	m.cursor = 15

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	if m.cursor != 0 {
		t.Error("cursor should reset on tab switch")
	}
}

func TestModel_PanelFocusSwitch(t *testing.T) {
	m := New(nil, 3, nil, "")
	if m.focusPanel != panelItems {
		t.Fatal("should start focused on items panel")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'h'})
	m = updated.(Model)
	if m.focusPanel != panelRepos {
		t.Error("h should switch focus to repos panel")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'l'})
	m = updated.(Model)
	if m.focusPanel != panelItems {
		t.Error("l should switch focus to items panel")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(Model)
	if m.focusPanel != panelRepos {
		t.Error("left arrow should switch focus to repos panel")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m = updated.(Model)
	if m.focusPanel != panelItems {
		t.Error("right arrow should switch focus to items panel")
	}
}

func TestModel_RepoSelection(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.issues = multiRepoItems(5)
	m.updateRepoList()

	m.focusPanel = panelRepos
	m.repoCursor = 1

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if m.selectedRepo == "" {
		t.Fatal("selectedRepo should be set")
	}
	items := m.currentItems()
	for _, item := range items {
		if item.Repo.String() != m.selectedRepo {
			t.Errorf("expected all items from %s, got %s", m.selectedRepo, item.Repo.String())
		}
	}
	if m.focusPanel != panelItems {
		t.Error("focus should return to items panel after selection")
	}
}

func TestModel_RepoSelectionAll(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.issues = multiRepoItems(5)
	m.updateRepoList()
	m.selectedRepo = m.repos[0]

	m.focusPanel = panelRepos
	m.repoCursor = 0

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if m.selectedRepo != "" {
		t.Error("selecting 'All' should clear selectedRepo")
	}
	items := m.currentItems()
	if len(items) != 10 {
		t.Errorf("expected 10 items with All selected, got %d", len(items))
	}
}

func TestModel_CollapsedGroups(t *testing.T) {
	m := New(nil, 2, nil, "")
	m.issues = multiRepoItems(5)
	m.updateRepoList()
	m.fullLoaded["a/alpha"] = true
	m.fullLoaded["b/beta"] = true

	rows := m.visibleItems()

	expandCount := 0
	itemCount := 0
	for _, r := range rows {
		if r.expandRepo != "" {
			expandCount++
		}
		if r.item != nil {
			itemCount++
		}
	}

	if itemCount != 4 {
		t.Errorf("expected 4 visible items (2 per repo), got %d", itemCount)
	}
	if expandCount != 2 {
		t.Errorf("expected 2 expand rows, got %d", expandCount)
	}
}

func TestModel_ExpandGroup(t *testing.T) {
	m := New(nil, 2, nil, "")
	m.issues = multiRepoItems(5)
	m.updateRepoList()
	m.fullLoaded["a/alpha"] = true
	m.fullLoaded["b/beta"] = true

	rows := m.visibleItems()
	expandIdx := -1
	for i, r := range rows {
		if r.expandRepo != "" {
			expandIdx = i
			break
		}
	}
	if expandIdx == -1 {
		t.Fatal("should have an expandable row")
	}

	m.cursor = expandIdx
	m.focusPanel = panelItems
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	rows = m.visibleItems()
	for _, r := range rows {
		if r.expandRepo == m.repos[0] {
			t.Error("first repo should no longer have expand row after expanding")
		}
	}
}

func TestModel_RepoListUpdated(t *testing.T) {
	m := New(nil, 3, nil, "")
	issues, prs := testItems()

	updated, _ := m.Update(itemsReceivedMsg{issues: issues, prs: prs})
	m = updated.(Model)

	if len(m.repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(m.repos))
	}
	if m.repos[0] != "o/r" {
		t.Errorf("expected repo o/r, got %s", m.repos[0])
	}
}

func TestModel_RepoPanelNavigation(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.issues = multiRepoItems(3)
	m.updateRepoList()
	m.focusPanel = panelRepos

	if m.repoCursor != 0 {
		t.Fatal("repo cursor should start at 0")
	}

	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
	m = updated.(Model)
	if m.repoCursor != 1 {
		t.Errorf("expected repoCursor 1, got %d", m.repoCursor)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	m = updated.(Model)
	if m.repoCursor != 2 {
		t.Errorf("expected repoCursor 2, got %d", m.repoCursor)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	m = updated.(Model)
	if m.repoCursor != 2 {
		t.Error("repo cursor should not go past last repo")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	m = updated.(Model)
	if m.repoCursor != 1 {
		t.Errorf("expected repoCursor 1, got %d", m.repoCursor)
	}
}

func TestModel_DefaultItemsPerRepo(t *testing.T) {
	m := New(nil, 0, nil, "")
	if m.itemsPerRepo != 3 {
		t.Errorf("expected default itemsPerRepo 3, got %d", m.itemsPerRepo)
	}
}

func TestModel_TabSwitchClearsExpanded(t *testing.T) {
	m := New(nil, 2, nil, "")
	m.issues = multiRepoItems(5)
	m.updateRepoList()
	m.expanded["a/alpha"] = true

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	if len(m.expanded) != 0 {
		t.Error("expanded map should be cleared on tab switch")
	}
}

func TestModel_DetectFullLoaded(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.issues = []provider.Item{
		{ID: "a/x/issues/1", Number: 1, Repo: provider.RepoRef{Owner: "a", Repo: "x"}},
		{ID: "a/x/issues/2", Number: 2, Repo: provider.RepoRef{Owner: "a", Repo: "x"}},
	}
	m.prs = []provider.Item{
		{ID: "a/x/prs/1", Number: 1, Repo: provider.RepoRef{Owner: "a", Repo: "x"}},
	}

	updated, _ := m.Update(itemsReceivedMsg{issues: m.issues, prs: m.prs})
	m = updated.(Model)

	if !m.fullLoaded["a/x"] {
		t.Error("repo with fewer items than limit should be marked fullLoaded")
	}
}

func TestModel_NotFullLoadedAtLimit(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.issues = multiRepoItems(3)

	updated, _ := m.Update(itemsReceivedMsg{issues: m.issues, prs: nil})
	m = updated.(Model)

	if m.fullLoaded["a/alpha"] {
		t.Error("repo with exactly itemsPerRepo items should NOT be marked fullLoaded")
	}
}

func TestModel_MoreLabelNotFullLoaded(t *testing.T) {
	m := New(nil, 2, nil, "")
	m.issues = multiRepoItems(2)
	m.updateRepoList()
	m.rebuildViewport()

	view := m.View()
	if !containsString(view.Content, "[more...]") {
		t.Error("should show [more...] for repos not fully loaded")
	}
}

func TestModel_MoreLabelFullLoaded(t *testing.T) {
	m := New(nil, 2, nil, "")
	m.issues = multiRepoItems(5)
	m.updateRepoList()
	m.fullLoaded["a/alpha"] = true
	m.fullLoaded["b/beta"] = true
	m.rebuildViewport()

	view := m.View()
	if !containsString(view.Content, "[+3 more]") {
		t.Error("should show [+3 more] for fully loaded repos with 5 items and limit 2")
	}
}

func TestModel_RepoExpandedMsg(t *testing.T) {
	m := New(nil, 2, nil, "")
	m.issues = multiRepoItems(2)
	m.updateRepoList()

	fullItems := make([]provider.Item, 5)
	for i := range 5 {
		fullItems[i] = provider.Item{
			ID:     fmt.Sprintf("a/alpha/issues/%d", i+1),
			Number: i + 1,
			Title:  fmt.Sprintf("Issue %d", i+1),
			Repo:   provider.RepoRef{Owner: "a", Repo: "alpha"},
		}
	}

	updated, _ := m.Update(repoExpandedMsg{
		repo:     "a/alpha",
		items:    fullItems,
		itemType: provider.ItemTypeIssue,
	})
	m = updated.(Model)

	if !m.fullLoaded["a/alpha"] {
		t.Error("repo should be marked fullLoaded after expand")
	}
	if !m.expanded["a/alpha"] {
		t.Error("repo should be auto-expanded after expand")
	}

	alphaCount := 0
	for _, item := range m.issues {
		if item.Repo.String() == "a/alpha" {
			alphaCount++
		}
	}
	if alphaCount != 5 {
		t.Errorf("expected 5 alpha items after expand, got %d", alphaCount)
	}
}

func TestModel_ChunkedExpand_NotFullWhenAtLimit(t *testing.T) {
	m := New(nil, 2, nil, "")
	m.height = 50
	m.issues = multiRepoItems(2)
	m.updateRepoList()

	chunkItems := make([]provider.Item, 8)
	for i := range 8 {
		chunkItems[i] = provider.Item{
			ID:     fmt.Sprintf("a/alpha/issues/%d", i+1),
			Number: i + 1,
			Title:  fmt.Sprintf("Issue %d", i+1),
			Repo:   provider.RepoRef{Owner: "a", Repo: "alpha"},
		}
	}

	updated, _ := m.Update(repoExpandedMsg{
		repo:           "a/alpha",
		items:          chunkItems,
		itemType:       provider.ItemTypeIssue,
		requestedLimit: 8,
	})
	m = updated.(Model)

	if m.fullLoaded["a/alpha"] {
		t.Error("repo should NOT be fullLoaded when returned items == requestedLimit")
	}
	if !m.expanded["a/alpha"] {
		t.Error("repo should be expanded after chunk load")
	}

	view := m.View()
	if !containsString(view.Content, "[more...]") {
		t.Error("should still show [more...] for expanded but not fully loaded repo")
	}
}

func TestModel_ChunkedExpand_FullWhenUnderLimit(t *testing.T) {
	m := New(nil, 2, nil, "")
	m.issues = multiRepoItems(2)
	m.updateRepoList()

	chunkItems := make([]provider.Item, 5)
	for i := range 5 {
		chunkItems[i] = provider.Item{
			ID:     fmt.Sprintf("a/alpha/issues/%d", i+1),
			Number: i + 1,
			Title:  fmt.Sprintf("Issue %d", i+1),
			Repo:   provider.RepoRef{Owner: "a", Repo: "alpha"},
		}
	}

	updated, _ := m.Update(repoExpandedMsg{
		repo:           "a/alpha",
		items:          chunkItems,
		itemType:       provider.ItemTypeIssue,
		requestedLimit: 8,
	})
	m = updated.(Model)

	if !m.fullLoaded["a/alpha"] {
		t.Error("repo should be fullLoaded when returned items < requestedLimit")
	}
}

func TestModel_ScrollToLastRow_MultiRepo(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.height = 16
	m.issues = multiRepoItems(3)
	m.updateRepoList()
	m.fullLoaded["a/alpha"] = true
	m.fullLoaded["b/beta"] = true

	rows := m.visibleItems()
	totalRows := len(rows)

	for range totalRows - 1 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
		m = updated.(Model)
	}

	if m.cursor != totalRows-1 {
		t.Errorf("cursor should reach the last row %d, got %d", totalRows-1, m.cursor)
	}

	view := m.View()
	lastRow := rows[totalRows-1]
	if lastRow.item != nil {
		if !containsString(view.Content, lastRow.item.Title) {
			t.Errorf("last item %q should be visible in the view", lastRow.item.Title)
		}
	}
}

func TestModel_ScrollToBottom_CursorVisible(t *testing.T) {
	m := New(nil, 2, nil, "")
	m.height = 12
	items := multiRepoItems(5)
	m.issues = items
	m.updateRepoList()
	m.fullLoaded["a/alpha"] = true
	m.fullLoaded["b/beta"] = true
	m.expanded["a/alpha"] = true
	m.expanded["b/beta"] = true

	rows := m.visibleItems()
	totalRows := len(rows)

	for range totalRows - 1 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
		m = updated.(Model)
	}

	if m.cursor != totalRows-1 {
		t.Errorf("should reach last row %d, got %d", totalRows-1, m.cursor)
	}

	view := m.View()
	lastRow := rows[totalRows-1]
	if lastRow.item != nil {
		if !containsString(view.Content, lastRow.item.Title) {
			t.Errorf("last item %q must be visible, cursor=%d", lastRow.item.Title, m.cursor)
		}
	}
}

func manyRepoItems(numRepos, perRepo int) []provider.Item {
	var items []provider.Item
	for r := range numRepos {
		repo := provider.RepoRef{Owner: "org", Repo: fmt.Sprintf("repo-%02d", r+1)}
		for i := range perRepo {
			items = append(items, provider.Item{
				ID:     fmt.Sprintf("%s/issues/%d", repo.String(), i+1),
				Number: i + 1,
				Title:  fmt.Sprintf("Issue %d in %s", i+1, repo.Repo),
				Type:   provider.ItemTypeIssue,
				Repo:   repo,
			})
		}
	}
	return items
}

func TestModel_ScrollToBottom_ManyRepos(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.height = 30
	m.width = 80
	m.issues = manyRepoItems(11, 3)
	m.updateRepoList()
	for _, repo := range m.repos {
		m.fullLoaded[repo] = true
	}

	rows := m.visibleItems()
	totalRows := len(rows)

	for i := range totalRows - 1 {
		updated, _ := m.Update(tea.KeyPressMsg{Code: 'j'})
		m = updated.(Model)

		view := m.View()
		currentRow := rows[m.cursor]
		var needle string
		if currentRow.item != nil {
			needle = currentRow.item.Title
		} else if currentRow.expandRepo != "" {
			needle = "more"
		}
		if needle != "" && !containsString(view.Content, needle) {
			t.Fatalf("step %d: cursor=%d — %q not visible in view", i+1, m.cursor, needle)
		}
	}

	if m.cursor != totalRows-1 {
		t.Errorf("should reach last row %d, got %d", totalRows-1, m.cursor)
	}
}

func testSessions() []protocol.SessionPayload {
	return []protocol.SessionPayload{
		{ID: "fix-1-abc", Repo: "o/r", Number: 1, ItemType: "issue", Agent: "claude", Status: "running", CreatedAt: time.Now().Add(-2 * time.Minute).Format(time.RFC3339)},
		{ID: "rev-10-xyz", Repo: "o/r", Number: 10, ItemType: "pr", Agent: "claude", Status: "completed", CreatedAt: time.Now().Add(-5 * time.Minute).Format(time.RFC3339)},
	}
}

func TestModel_EnterOpensItemDetail(t *testing.T) {
	m := New(nil, 3, nil, "")
	issues, _ := testItems()
	m.issues = issues
	m.cursor = 0

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if m.view != viewItemDetail {
		t.Error("Enter on item should open item detail view")
	}
	if m.itemDetail == nil {
		t.Error("itemDetail should be set")
	}
}

func TestModel_ItemDetailEscapeReturns(t *testing.T) {
	m := New(nil, 3, nil, "")
	issues, _ := testItems()
	m.issues = issues
	m.cursor = 0

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if m.view != viewItemDetail {
		t.Fatal("should be in item detail view")
	}

	updated, _ = m.Update(attach.CloseMsg{})
	m = updated.(Model)

	if m.view != viewOverview {
		t.Error("CloseMsg should return to overview")
	}
	if m.itemDetail != nil {
		t.Error("itemDetail should be nil after close")
	}
}

func TestModel_AttachEnterOnRunningSession(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.sessions = testSessions()
	m.activeTab = tabSessions
	m.sessionCursor = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Enter on running session should return tmux attach command")
	}
	if m.attachedSession != nil {
		t.Error("attachedSession should not be set for running sessions (uses tmux attach)")
	}
}

func TestModel_AttachEnterOnCompletedSession(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.sessions = testSessions()
	m.activeTab = tabSessions
	m.sessionCursor = 1

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if m.attachedSession == nil {
		t.Error("should attach to completed session for read-only viewing")
	}
}

func TestModel_AttachViewEntersOnMsg(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.sessions = testSessions()
	m.attachedSession = &m.sessions[0]

	updated, _ := m.Update(sessionAttachedMsg{})
	m = updated.(Model)

	if m.view != viewAttach {
		t.Error("should be in attach view after sessionAttachedMsg")
	}
}

func TestModel_AttachOutputAppends(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.view = viewAttach
	m.attachedSession = &protocol.SessionPayload{ID: "fix-1-abc", Status: "running"}
	m.width = 80
	m.height = 40

	updated, _ := m.Update(sessionOutputMsg{sessionID: "fix-1-abc", data: "line 1\nline 2"})
	m = updated.(Model)

	if len(m.attachOutput) != 1 {
		t.Fatalf("expected 1 output chunk, got %d", len(m.attachOutput))
	}
	if m.attachOutput[0] != "line 1\nline 2" {
		t.Errorf("unexpected output: %q", m.attachOutput[0])
	}

	updated, _ = m.Update(sessionOutputMsg{sessionID: "fix-1-abc", data: "screen update"})
	m = updated.(Model)

	if len(m.attachOutput) != 1 {
		t.Fatalf("expected 1 output chunk (replace), got %d", len(m.attachOutput))
	}
	if m.attachOutput[0] != "screen update" {
		t.Errorf("expected replaced output, got %q", m.attachOutput[0])
	}
}

func TestModel_AttachViewRender(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.view = viewAttach
	m.width = 80
	m.height = 30
	m.attachedSession = &protocol.SessionPayload{
		ID: "fix-1-abc", Repo: "o/r", Number: 1, Agent: "claude", Status: "completed",
	}
	m.attachOutput = []string{"hello world"}
	m.rebuildAttachViewport()

	view := m.View()
	if !containsString(view.Content, "fix-1-abc") {
		t.Error("attach view should show session ID")
	}
	if !containsString(view.Content, "o/r#1") {
		t.Error("attach view should show repo and number")
	}
	if !containsString(view.Content, "hello world") {
		t.Error("attach view should show output")
	}
	if !containsString(view.Content, "esc: back") {
		t.Error("attach view should show help text")
	}
}

func TestModel_AttachEscapeDetaches(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.view = viewAttach
	m.attachedSession = &protocol.SessionPayload{ID: "fix-1-abc", Status: "completed"}
	m.attachOutput = []string{"some output"}

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)

	if m.view != viewOverview {
		t.Error("should return to overview after Escape")
	}
	if m.attachedSession != nil {
		t.Error("attachedSession should be nil after Escape")
	}
	if m.attachOutput != nil {
		t.Error("attachOutput should be nil after Escape")
	}
}

func TestModel_AttachNonOutputIgnored(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.view = viewAttach
	m.attachedSession = &protocol.SessionPayload{ID: "fix-1-abc", Status: "running"}

	updated, _ := m.Update(attachNonOutputMsg{})
	m = updated.(Model)

	if m.view != viewAttach {
		t.Error("should stay in attach view after non-output msg")
	}
}

func TestModel_AttachOutputIgnoredInOverview(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.view = viewOverview

	updated, _ := m.Update(sessionOutputMsg{sessionID: "fix-1-abc", data: "stale output"})
	m = updated.(Model)

	if len(m.attachOutput) != 0 {
		t.Error("should not append output when in overview mode")
	}
}

func TestModel_AttachQuitDoesNotQuit(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.view = viewAttach
	m.attachedSession = &protocol.SessionPayload{ID: "fix-1-abc", Status: "completed"}

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})

	if cmd != nil {
		t.Error("q should not quit in attach mode")
	}
}

func TestModel_ItemDetailScrollDoesNotOpenAgentPicker(t *testing.T) {
	m := New(nil, 3, []string{"claude", "gpt"}, "")
	issues, _ := testItems()
	m.issues = issues
	m.width = 80
	m.height = 40
	m.cursor = 0

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if m.view != viewItemDetail {
		t.Fatalf("expected viewItemDetail after Enter, got %d", m.view)
	}
	if m.itemDetail == nil {
		t.Fatal("itemDetail should be set")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)

	if m.view != viewItemDetail {
		t.Errorf("expected viewItemDetail after arrow down, got %d (viewAgentPicker=%d)", m.view, viewAgentPicker)
	}
	if m.itemDetail == nil {
		t.Error("itemDetail should still be set after arrow down")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(Model)
	if m.view != viewItemDetail {
		t.Errorf("expected viewItemDetail after second arrow down, got %d", m.view)
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m = updated.(Model)
	if m.view != viewItemDetail {
		t.Errorf("expected viewItemDetail after arrow up, got %d", m.view)
	}
}

func TestModel_ItemDetailAKeySpawnsAgent(t *testing.T) {
	m := New(nil, 3, []string{"claude"}, "")
	issues, _ := testItems()
	m.issues = issues
	m.width = 80
	m.height = 40
	m.cursor = 0

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if m.view != viewItemDetail {
		t.Fatal("should be in item detail view")
	}

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'a'})
	m = updated.(Model)

	if cmd == nil {
		t.Fatal("pressing 'a' in detail view should return a cmd producing SpawnSessionMsg")
	}

	msg := cmd()
	if _, ok := msg.(attach.SpawnSessionMsg); !ok {
		t.Errorf("expected SpawnSessionMsg, got %T", msg)
	}
}

func TestModel_TmuxDetachedReturnsToOverview(t *testing.T) {
	m := New(nil, 3, nil, "")
	m.view = viewOverview
	m.sessions = []protocol.SessionPayload{
		{ID: "fix-1-abc", Status: "running"},
	}

	updated, _ := m.Update(tmuxDetachedMsg{})
	m = updated.(Model)

	if m.view != viewOverview {
		t.Error("should be in overview after tmux detach")
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && contains(s, substr)
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

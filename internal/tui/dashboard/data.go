package dashboard

import (
	"sort"
	"strings"

	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

func (m Model) currentItems() []provider.Item {
	var all []provider.Item
	if m.activeTab == tabIssues {
		all = m.issues
	} else {
		all = m.prs
	}

	if m.selectedRepo == "" {
		return all
	}

	var filtered []provider.Item
	for _, item := range all {
		if item.Repo.String() == m.selectedRepo {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m Model) filteredItems() []provider.Item {
	items := m.currentItems()
	if m.searchActive || m.searchCommitted {
		items = filterItems(items, m.searchInput)
	}
	return items
}

func (m Model) filteredJiraItems() []provider.JiraItem {
	if m.searchActive || m.searchCommitted {
		return filterJiraItems(m.jiraItems, m.searchInput)
	}
	return m.jiraItems
}

func (m Model) filteredSessions() []protocol.SessionPayload {
	if m.searchActive || m.searchCommitted {
		return filterSessions(m.sessions, m.searchInput)
	}
	return m.sessions
}

func (m Model) visibleItems() []visibleRow {
	items := m.filteredItems()
	return buildVisibleRows(items, m.itemsPerRepo, m.expanded, m.selectedRepo, m.fullLoaded)
}

func (m Model) selectedItem() *provider.Item {
	rows := m.visibleItems()
	if m.cursor >= 0 && m.cursor < len(rows) {
		r := rows[m.cursor]
		if r.item != nil {
			return r.item
		}
	}
	return nil
}

func (m Model) expandableRepoAtCursor() string {
	rows := m.visibleItems()
	if m.cursor >= 0 && m.cursor < len(rows) {
		return rows[m.cursor].expandRepo
	}
	return ""
}

func (m *Model) updateRepoList() {
	seen := make(map[string]bool)
	for _, item := range m.issues {
		seen[item.Repo.String()] = true
	}
	for _, item := range m.prs {
		seen[item.Repo.String()] = true
	}
	m.repos = make([]string, 0, len(seen))
	for repo := range seen {
		m.repos = append(m.repos, repo)
	}
	sort.Strings(m.repos)
}

func (m *Model) detectFullLoaded() {
	repoIssueCount := make(map[string]int)
	for _, item := range m.issues {
		repoIssueCount[item.Repo.String()]++
	}
	repoPRCount := make(map[string]int)
	for _, item := range m.prs {
		repoPRCount[item.Repo.String()]++
	}
	for _, repo := range m.repos {
		if repoIssueCount[repo] < m.itemsPerRepo && repoPRCount[repo] < m.itemsPerRepo {
			m.fullLoaded[repo] = true
		}
	}
}

func (m *Model) mergeExpandedItems(msg repoExpandedMsg) {
	if msg.itemType == provider.ItemTypeIssue {
		m.issues = replaceRepoItems(m.issues, msg.repo, msg.items)
	} else {
		m.prs = replaceRepoItems(m.prs, msg.repo, msg.items)
	}
	if msg.requestedLimit <= 0 || len(msg.items) < msg.requestedLimit {
		m.fullLoaded[msg.repo] = true
	}
	m.expanded[msg.repo] = true
	m.updateRepoList()
}

func replaceRepoItems(all []provider.Item, repo string, newItems []provider.Item) []provider.Item {
	var kept []provider.Item
	for _, item := range all {
		if item.Repo.String() != repo {
			kept = append(kept, item)
		}
	}
	return append(kept, newItems...)
}

func (m Model) updateItem(items []provider.Item, updated provider.Item) {
	for i, item := range items {
		if item.ID == updated.ID {
			items[i] = updated
			return
		}
	}
}

func (m Model) selectedJiraItem() *provider.JiraItem {
	items := m.filteredJiraItems()
	if m.jiraCursor >= 0 && m.jiraCursor < len(items) {
		return &items[m.jiraCursor]
	}
	return nil
}

func (m Model) allRepoAtCursor() string {
	rows := m.visibleItems()
	if m.cursor >= 0 && m.cursor < len(rows) {
		return rows[m.cursor].allRepo
	}
	return ""
}

func filterItems(items []provider.Item, query string) []provider.Item {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	var filtered []provider.Item
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Title), q) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterJiraItems(items []provider.JiraItem, query string) []provider.JiraItem {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)
	var filtered []provider.JiraItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Key), q) || strings.Contains(strings.ToLower(item.Summary), q) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterSessions(sessions []protocol.SessionPayload, query string) []protocol.SessionPayload {
	if query == "" {
		return sessions
	}
	q := strings.ToLower(query)
	var filtered []protocol.SessionPayload
	for _, s := range sessions {
		name := s.Name
		if name == "" {
			name = s.ID
		}
		if strings.Contains(strings.ToLower(name), q) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

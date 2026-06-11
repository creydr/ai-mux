package dashboard

import (
	"sort"

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

func (m Model) visibleItems() []visibleRow {
	items := m.currentItems()
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
	if m.jiraCursor >= 0 && m.jiraCursor < len(m.jiraItems) {
		return &m.jiraItems[m.jiraCursor]
	}
	return nil
}

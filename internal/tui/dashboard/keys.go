package dashboard

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/tui/attach"
)

func contextPromptForItem(item *provider.Item) string {
	if item == nil || item.URL == "" {
		return ""
	}
	repo := item.Repo.String()
	if item.Type == provider.ItemTypePR {
		return fmt.Sprintf("You are working on GitHub PR %s. Use \"gh pr view %d --repo %s\" to inspect the PR details and provide a review.", item.URL, item.Number, repo)
	}
	return fmt.Sprintf("You are working on GitHub issue %s. Use \"gh issue view %d --repo %s\" to inspect the issue details and then provide a fix.", item.URL, item.Number, repo)
}

func contextPromptForJiraItem(item *provider.JiraItem) string {
	if item == nil {
		return ""
	}
	return fmt.Sprintf("You are working on Jira item %s (%s). Use \"acli jira workitem view %s\" to inspect the item details and then provide a fix.", item.Key, item.URL, item.Key)
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.Code == 'c' && msg.Mod.Contains(tea.ModCtrl) {
		return m, tea.Quit
	}
	if m.searchActive {
		return m.handleSearchKey(msg)
	}
	if m.renameActive {
		return m.handleRenameKey(msg)
	}
	if m.view == viewAttach {
		return m.handleAttachKey(msg)
	}
	if m.view == viewAgentPicker {
		return m.handleAgentPickerKey(msg)
	}
	if m.view == viewWorktreeChoice {
		return m.handleWorktreeChoiceKey(msg)
	}
	if m.view == viewSessionPicker {
		return m.handleSessionPickerKey(msg)
	}
	if m.view == viewRepoPicker {
		return m.handleRepoPickerKey(msg)
	}
	if m.view == viewHelp {
		return m.handleHelpKey(msg)
	}
	if m.view == viewItemDetail && m.itemDetail != nil {
		updated, cmd := m.itemDetail.Update(msg)
		detail := updated.(attach.Model)
		m.itemDetail = &detail
		return m, cmd
	}

	if m.searchCommitted && msg.Code == tea.KeyEscape {
		m.searchCommitted = false
		m.searchInput = ""
		m.rebuildViewport()
		return m, nil
	}

	switch {
	case msg.Code == 'h' || msg.Code == tea.KeyLeft:
		m.focusPanel = panelRepos
		return m, nil
	case msg.Code == 'l' || msg.Code == tea.KeyRight:
		m.focusPanel = panelItems
		return m, nil
	case msg.Code == tea.KeyTab:
		idx := 0
		for i, t := range m.enabledTabs {
			if t == m.activeTab {
				idx = i
				break
			}
		}
		idx = (idx + 1) % len(m.enabledTabs)
		m.activeTab = m.enabledTabs[idx]
		m.cursor = 0
		m.sessionCursor = 0
		m.expanded = make(map[string]bool)
		m.searchActive = false
		m.searchCommitted = false
		m.searchInput = ""
		switch m.activeTab {
		case tabIssues:
			m.issueBadge = 0
		case tabPRs:
			m.prBadge = 0
		case tabJira:
			m.jiraBadge = 0
			m.jiraCursor = 0
			if m.conn != nil {
				m.rebuildViewport()
				return m, fetchJiraItemsCmd(m.conn, 0, 0)
			}
		case tabSessions:
			m.sessionBadge = 0
			m.sessionScrollPos = 0
			if m.conn != nil {
				m.rebuildViewport()
				return m, tea.Batch(fetchSessionsCmd(m.conn), m.startSessionTick())
			}
		}
		m.rebuildViewport()
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		if m.activeTab == tabSessions {
			if m.sessionCursor < len(m.sessions)-1 {
				m.sessionCursor++
				m.sessionScrollPos = 0
			}
			m.rebuildViewport()
		} else if m.activeTab == tabJira {
			maxIdx := len(m.jiraItems) - 1
			if m.jiraHasMore {
				maxIdx += 2
			}
			if m.jiraCursor < maxIdx {
				m.jiraCursor++
			}
			m.rebuildViewport()
		} else if m.focusPanel == panelRepos {
			maxIdx := len(m.repos)
			if m.repoCursor < maxIdx {
				m.repoCursor++
			}
		} else {
			items := m.visibleItems()
			if m.cursor < len(items)-1 {
				m.cursor++
			}
			m.rebuildViewport()
		}
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
		if m.activeTab == tabSessions {
			if m.sessionCursor > 0 {
				m.sessionCursor--
				m.sessionScrollPos = 0
			}
			m.rebuildViewport()
		} else if m.activeTab == tabJira {
			if m.jiraCursor > 0 {
				m.jiraCursor--
			}
			m.rebuildViewport()
		} else if m.focusPanel == panelRepos {
			if m.repoCursor > 0 {
				m.repoCursor--
			}
		} else {
			if m.cursor > 0 {
				m.cursor--
			}
			m.rebuildViewport()
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.activeTab == tabJira {
			if m.jiraCursor == len(m.jiraItems) && m.jiraHasMore {
				return m, fetchJiraItemsCmd(m.conn, m.jiraOffset, 0)
			}
			if m.jiraCursor == len(m.jiraItems)+1 && m.jiraHasMore {
				m.loadingAllJira = true
				m.rebuildViewport()
				return m, fetchJiraItemsCmd(m.conn, m.jiraOffset, 0)
			}
			item := m.selectedJiraItem()
			if item != nil && m.conn != nil {
				detail := attach.NewEmbeddedJira(m.conn, item.Key, m.width, m.height, item)
				m.itemDetail = &detail
				m.view = viewItemDetail
				return m, m.itemDetail.Init()
			}
			return m, nil
		}
		if m.activeTab == tabSessions {
			if m.sessionCursor >= 0 && m.sessionCursor < len(m.sessions) {
				sess := m.sessions[m.sessionCursor]
				if sess.Status == "running" || sess.Status == "pending" {
					return m, tmuxAttachCmd(sess.ID, sess.Name)
				}
				m.attachedSession = &sess
				if m.conn != nil {
					return m, attachSessionCmd(m.conn, sess.ID)
				}
			}
			return m, nil
		}
		if m.focusPanel == panelRepos {
			if m.repoCursor == 0 {
				m.selectedRepo = ""
			} else {
				m.selectedRepo = m.repos[m.repoCursor-1]
			}
			m.cursor = 0
			m.expanded = make(map[string]bool)
			m.focusPanel = panelItems
			m.rebuildViewport()
			return m, nil
		}
		repo := m.expandableRepoAtCursor()
		if repo != "" {
			if !m.fullLoaded[repo] && m.conn != nil {
				itemType := provider.ItemTypeIssue
				var items []provider.Item
				if m.activeTab == tabPRs {
					itemType = provider.ItemTypePR
					items = m.prs
				} else {
					items = m.issues
				}
				currentCount := countRepoItems(items, repo)
				return m, expandRepoCmd(m.conn, repo, itemType, currentCount+expandChunkSize)
			}
			m.expanded[repo] = true
			m.rebuildViewport()
			return m, nil
		}
		allRepo := m.allRepoAtCursor()
		if allRepo != "" {
			if !m.fullLoaded[allRepo] && m.conn != nil {
				m.loadingAllRepos[allRepo] = true
				itemType := provider.ItemTypeIssue
				if m.activeTab == tabPRs {
					itemType = provider.ItemTypePR
				}
				m.rebuildViewport()
				return m, expandRepoCmd(m.conn, allRepo, itemType, 0)
			}
			return m, nil
		}
		item := m.selectedItem()
		if item != nil {
			ref := attach.Ref{
				Type:   item.Type,
				Owner:  item.Repo.Owner,
				Repo:   item.Repo.Repo,
				Number: item.Number,
			}
			detail := attach.NewEmbedded(m.conn, ref, m.width, m.height, item)
			m.itemDetail = &detail
			m.view = viewItemDetail
			return m, m.itemDetail.Init()
		}
		return m, nil
	case msg.Code == 'o' || msg.Code == 'b':
		if m.activeTab == tabJira {
			item := m.selectedJiraItem()
			if item != nil && item.URL != "" {
				return m, openBrowserCmd(item.URL)
			}
			return m, nil
		}
		item := m.selectedItem()
		if item != nil && item.URL != "" {
			return m, openBrowserCmd(item.URL)
		}
		return m, nil
	case msg.Code == 'a':
		if m.activeTab == tabSessions {
			return m, nil
		}
		if len(m.agents) == 0 {
			m.statusText = "No agents configured"
			return m, m.scheduleStatusClear()
		}
		if m.activeTab == tabJira {
			item := m.selectedJiraItem()
			if item == nil {
				return m, nil
			}
			cp := contextPromptForJiraItem(item)
			if len(m.configuredRepos) == 1 {
				req := &spawnRequest{repo: m.configuredRepos[0], itemType: string(provider.ItemTypeJira), itemKey: item.Key, contextPrompt: cp}
				if len(m.agents) == 1 {
					return m, spawnJiraSessionCmd(m.conn, req.repo, req.itemKey, m.agents[0], "", req.contextPrompt)
				}
				m.pendingSpawn = req
				m.agentCursor = 0
				m.view = viewAgentPicker
				return m, nil
			}
			m.pendingSpawn = &spawnRequest{itemType: string(provider.ItemTypeJira), itemKey: item.Key, contextPrompt: cp}
			m.repoPickerCursor = 0
			m.repoPickerActive = true
			m.view = viewRepoPicker
			return m, nil
		}
		item := m.selectedItem()
		if item == nil {
			return m, nil
		}
		itemType := string(provider.ItemTypeIssue)
		if m.activeTab == tabPRs {
			itemType = string(provider.ItemTypePR)
		}
		cp := contextPromptForItem(item)
		req := &spawnRequest{repo: item.Repo.String(), number: item.Number, itemType: itemType, contextPrompt: cp}
		if len(m.agents) == 1 {
			return m, spawnSessionCmd(m.conn, req.repo, req.number, req.itemType, m.agents[0], "", req.contextPrompt)
		}
		m.pendingSpawn = req
		m.agentCursor = 0
		m.view = viewAgentPicker
		return m, nil
	case msg.Code == 's':
		if m.activeTab == tabSessions {
			if m.sessionCursor >= 0 && m.sessionCursor < len(m.sessions) {
				sess := m.sessions[m.sessionCursor]
				if sess.Status == "running" || sess.Status == "pending" {
					return m, stopSessionCmd(m.conn, sess.ID)
				}
			}
		} else if m.activeTab == tabJira {
			item := m.selectedJiraItem()
			if item != nil {
				for _, sess := range m.sessions {
					if sess.ItemKey == item.Key && (sess.Status == "running" || sess.Status == "pending") {
						return m, stopSessionCmd(m.conn, sess.ID)
					}
				}
			}
		} else {
			item := m.selectedItem()
			if item != nil {
				for _, sess := range m.sessions {
					if sess.Repo == item.Repo.String() && sess.Number == item.Number && (sess.Status == "running" || sess.Status == "pending") {
						return m, stopSessionCmd(m.conn, sess.ID)
					}
				}
			}
		}
		return m, nil
	case msg.Code == 'n':
		if m.activeTab == tabSessions && m.sessionCursor >= 0 && m.sessionCursor < len(m.sessions) {
			sess := m.sessions[m.sessionCursor]
			m.renameActive = true
			m.renamingSession = sess.ID
			m.renameInput = sess.Name
			return m, nil
		}
		return m, nil
	case msg.Code == 't':
		if m.activeTab == tabJira {
			item := m.selectedJiraItem()
			if item == nil {
				return m, nil
			}
			var matched []protocol.SessionPayload
			for _, sess := range m.sessions {
				if sess.ItemKey == item.Key {
					matched = append(matched, sess)
				}
			}
			if len(matched) == 0 {
				m.statusText = "No sessions for this item"
				return m, m.scheduleStatusClear()
			}
			if len(matched) == 1 {
				sess := matched[0]
				if sess.Status == "running" || sess.Status == "pending" {
					return m, tmuxAttachCmd(sess.ID, sess.Name)
				}
				m.attachedSession = &sess
				if m.conn != nil {
					return m, attachSessionCmd(m.conn, sess.ID)
				}
				return m, nil
			}
			m.sessionPickerItems = matched
			m.sessionPickerCursor = 0
			m.view = viewSessionPicker
			return m, nil
		}
		if m.activeTab != tabSessions {
			item := m.selectedItem()
			if item == nil {
				return m, nil
			}
			repo := item.Repo.String()
			var matched []protocol.SessionPayload
			for _, sess := range m.sessions {
				if sess.Repo == repo && sess.Number == item.Number {
					matched = append(matched, sess)
				}
			}
			if len(matched) == 0 {
				m.statusText = "No sessions for this item"
				return m, m.scheduleStatusClear()
			}
			if len(matched) == 1 {
				sess := matched[0]
				if sess.Status == "running" || sess.Status == "pending" {
					return m, tmuxAttachCmd(sess.ID, sess.Name)
				}
				m.attachedSession = &sess
				if m.conn != nil {
					return m, attachSessionCmd(m.conn, sess.ID)
				}
				return m, nil
			}
			m.sessionPickerItems = matched
			m.sessionPickerCursor = 0
			m.view = viewSessionPicker
			return m, nil
		}
		return m, nil
	case msg.Code == 'r':
		if m.conn != nil {
			if m.activeTab == tabJira {
				return m, fetchJiraItemsCmd(m.conn, 0, 0)
			}
			m.fullLoaded = make(map[string]bool)
			return m, fetchItemsCmd(m.conn, m.itemsPerRepo)
		}
		return m, nil
	case msg.Code == ':':
		m.searchActive = true
		m.searchInput = ""
		m.searchCommitted = false
		m.preCursorPos = m.cursor
		m.preJiraCursor = m.jiraCursor
		m.preSessionCursor = m.sessionCursor
		m.rebuildViewport()
		return m, nil
	case msg.Code == '?':
		m.view = viewHelp
		return m, nil
	}
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyTab:
		m.searchActive = false
		m.searchCommitted = false
		m.searchInput = ""
		return m.handleKey(msg)
	case msg.Code == tea.KeyEscape:
		m.searchActive = false
		m.searchCommitted = false
		m.searchInput = ""
		m.cursor = m.preCursorPos
		m.jiraCursor = m.preJiraCursor
		m.sessionCursor = m.preSessionCursor
		m.rebuildViewport()
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.searchInput == "" {
			m.searchActive = false
			m.searchCommitted = false
		} else {
			m.searchActive = false
			m.searchCommitted = true
		}
		m.rebuildViewport()
		return m, nil
	case msg.Code == tea.KeyBackspace:
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}
		m.clampSearchCursor()
		m.rebuildViewport()
		return m, nil
	case msg.Code == tea.KeyUp:
		m.moveSearchCursor(-1)
		m.rebuildViewport()
		return m, nil
	case msg.Code == tea.KeyDown:
		m.moveSearchCursor(1)
		m.rebuildViewport()
		return m, nil
	default:
		if msg.Text != "" {
			m.searchInput += msg.Text
			m.clampSearchCursor()
			m.rebuildViewport()
		}
		return m, nil
	}
}

func (m *Model) clampSearchCursor() {
	switch m.activeTab {
	case tabSessions:
		maxIdx := len(m.filteredSessions()) - 1
		if maxIdx < 0 {
			maxIdx = 0
		}
		if m.sessionCursor > maxIdx {
			m.sessionCursor = maxIdx
		}
	case tabJira:
		maxIdx := len(m.filteredJiraItems()) - 1
		if maxIdx < 0 {
			maxIdx = 0
		}
		if m.jiraCursor > maxIdx {
			m.jiraCursor = maxIdx
		}
	default:
		maxIdx := len(m.visibleItems()) - 1
		if maxIdx < 0 {
			maxIdx = 0
		}
		if m.cursor > maxIdx {
			m.cursor = maxIdx
		}
	}
}

func (m *Model) moveSearchCursor(delta int) {
	switch m.activeTab {
	case tabSessions:
		m.sessionCursor += delta
		maxIdx := len(m.filteredSessions()) - 1
		if m.sessionCursor < 0 {
			m.sessionCursor = 0
		}
		if maxIdx >= 0 && m.sessionCursor > maxIdx {
			m.sessionCursor = maxIdx
		}
	case tabJira:
		m.jiraCursor += delta
		maxIdx := len(m.filteredJiraItems()) - 1
		if m.jiraCursor < 0 {
			m.jiraCursor = 0
		}
		if maxIdx >= 0 && m.jiraCursor > maxIdx {
			m.jiraCursor = maxIdx
		}
	default:
		m.cursor += delta
		maxIdx := len(m.visibleItems()) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
		if maxIdx >= 0 && m.cursor > maxIdx {
			m.cursor = maxIdx
		}
	}
}

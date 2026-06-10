package dashboard

import (
	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/tui/attach"
)

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.Code == 'c' && msg.Mod.Contains(tea.ModCtrl) {
		return m, tea.Quit
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
	if m.view == viewHelp {
		return m.handleHelpKey(msg)
	}
	if m.view == viewItemDetail && m.itemDetail != nil {
		updated, cmd := m.itemDetail.Update(msg)
		detail := updated.(attach.Model)
		m.itemDetail = &detail
		return m, cmd
	}

	switch {
	case msg.Code == 'h' || msg.Code == tea.KeyLeft:
		m.focusPanel = panelRepos
		return m, nil
	case msg.Code == 'l' || msg.Code == tea.KeyRight:
		m.focusPanel = panelItems
		return m, nil
	case msg.Code == tea.KeyTab:
		m.activeTab = (m.activeTab + 1) % tab(len(tabNames))
		m.cursor = 0
		m.sessionCursor = 0
		m.expanded = make(map[string]bool)
		switch m.activeTab {
		case tabIssues:
			m.issueBadge = 0
		case tabPRs:
			m.prBadge = 0
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
		item := m.selectedItem()
		if item == nil {
			return m, nil
		}
		itemType := string(provider.ItemTypeIssue)
		if m.activeTab == tabPRs {
			itemType = string(provider.ItemTypePR)
		}
		req := &spawnRequest{repo: item.Repo.String(), number: item.Number, itemType: itemType}
		if m.defaultAgent != "" {
			return m, spawnSessionCmd(m.conn, req.repo, req.number, req.itemType, m.defaultAgent, "")
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
			m.fullLoaded = make(map[string]bool)
			return m, fetchItemsCmd(m.conn, m.itemsPerRepo)
		}
		return m, nil
	case msg.Code == '?':
		m.view = viewHelp
		return m, nil
	}
	return m, nil
}

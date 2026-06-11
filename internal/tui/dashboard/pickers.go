package dashboard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Agent picker

func (m Model) handleAgentPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape || msg.Code == 'q':
		m.view = viewOverview
		m.pendingSpawn = nil
		m.rebuildViewport()
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		if m.agentCursor < len(m.agents)-1 {
			m.agentCursor++
		}
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
		if m.agentCursor > 0 {
			m.agentCursor--
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.pendingSpawn != nil && m.agentCursor < len(m.agents) && m.conn != nil {
			agent := m.agents[m.agentCursor]
			req := m.pendingSpawn
			m.pendingSpawn = nil
			m.view = viewOverview
			m.rebuildViewport()
			if req.itemKey != "" {
				return m, spawnJiraSessionCmd(m.conn, req.repo, req.itemKey, agent, "", req.contextPrompt)
			}
			return m, spawnSessionCmd(m.conn, req.repo, req.number, req.itemType, agent, "", req.contextPrompt)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderAgentPicker() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Select Agent"))
	b.WriteString("\n\n")

	if m.pendingSpawn != nil {
		if m.pendingSpawn.itemKey != "" {
			b.WriteString(fmt.Sprintf("  Spawning session for %s in %s\n\n", m.pendingSpawn.itemKey, m.pendingSpawn.repo))
		} else {
			b.WriteString(fmt.Sprintf("  Spawning session for %s#%d\n\n", m.pendingSpawn.repo, m.pendingSpawn.number))
		}
	}

	for i, name := range m.agents {
		if i == m.agentCursor {
			b.WriteString(selectedItemStyle.Render("> "+name) + "\n")
		} else {
			b.WriteString("  " + name + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("  enter: select | esc: cancel"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// Worktree choice

var worktreeChoices = []struct {
	label  string
	action string
}{
	{"Resume in existing worktree", "reuse"},
	{"Start fresh (remove old worktree)", "fresh"},
	{"Create new (keep old worktree)", "new"},
}

func (m Model) handleWorktreeChoiceKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape || msg.Code == 'q':
		m.view = viewOverview
		m.pendingSpawn = nil
		m.rebuildViewport()
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		if m.worktreeChoiceIdx < len(worktreeChoices)-1 {
			m.worktreeChoiceIdx++
		}
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
		if m.worktreeChoiceIdx > 0 {
			m.worktreeChoiceIdx--
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.pendingSpawn != nil && m.conn != nil {
			req := m.pendingSpawn
			action := worktreeChoices[m.worktreeChoiceIdx].action
			m.pendingSpawn = nil
			m.view = viewOverview
			m.rebuildViewport()
			if req.itemKey != "" {
				return m, spawnJiraSessionCmd(m.conn, req.repo, req.itemKey, req.agent, action, req.contextPrompt)
			}
			return m, spawnSessionCmd(m.conn, req.repo, req.number, req.itemType, req.agent, action, req.contextPrompt)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderWorktreeChoice() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Worktree Already Exists"))
	b.WriteString("\n\n")

	if m.pendingSpawn != nil {
		b.WriteString(fmt.Sprintf("  A worktree for %s#%d already exists.\n", m.pendingSpawn.repo, m.pendingSpawn.number))
		b.WriteString("  What would you like to do?\n\n")
	}

	for i, choice := range worktreeChoices {
		if i == m.worktreeChoiceIdx {
			b.WriteString(selectedItemStyle.Render("> "+choice.label) + "\n")
		} else {
			b.WriteString("  " + choice.label + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("  enter: select | esc: cancel"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// Session picker

func (m Model) handleSessionPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape || msg.Code == 'q':
		m.view = viewOverview
		m.sessionPickerItems = nil
		m.rebuildViewport()
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		if m.sessionPickerCursor < len(m.sessionPickerItems)-1 {
			m.sessionPickerCursor++
		}
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
		if m.sessionPickerCursor > 0 {
			m.sessionPickerCursor--
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.sessionPickerCursor < len(m.sessionPickerItems) {
			sess := m.sessionPickerItems[m.sessionPickerCursor]
			m.view = viewOverview
			m.sessionPickerItems = nil
			m.rebuildViewport()
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
	return m, nil
}

func (m Model) renderSessionPicker() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Select Session"))
	b.WriteString("\n\n")

	for i, s := range m.sessionPickerItems {
		label := s.ID
		if s.Name != "" {
			label = s.Name + " (" + s.ID + ")"
		}
		badge := sessionBadge(s.Status, s.WaitingInput)
		entry := fmt.Sprintf("%s  %s  %s", label, s.Agent, badge)
		if i == m.sessionPickerCursor {
			b.WriteString(selectedItemStyle.Render("> "+entry) + "\n")
		} else {
			b.WriteString("  " + entry + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("  enter: select | esc: cancel"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// Repo picker

func (m Model) handleRepoPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape || msg.Code == 'q':
		m.view = viewOverview
		m.repoPickerActive = false
		m.pendingSpawn = nil
		m.rebuildViewport()
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		if m.repoPickerCursor < len(m.configuredRepos)-1 {
			m.repoPickerCursor++
		}
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
		if m.repoPickerCursor > 0 {
			m.repoPickerCursor--
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.pendingSpawn != nil && m.repoPickerCursor < len(m.configuredRepos) {
			m.pendingSpawn.repo = m.configuredRepos[m.repoPickerCursor]
			m.repoPickerActive = false
			if m.defaultAgent != "" {
				req := m.pendingSpawn
				m.pendingSpawn = nil
				m.view = viewOverview
				m.rebuildViewport()
				return m, spawnJiraSessionCmd(m.conn, req.repo, req.itemKey, m.defaultAgent, "", req.contextPrompt)
			}
			m.agentCursor = 0
			m.view = viewAgentPicker
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderRepoPicker() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Select Repository"))
	b.WriteString("\n\n")

	if m.pendingSpawn != nil && m.pendingSpawn.itemKey != "" {
		b.WriteString(fmt.Sprintf("  Spawning session for %s\n\n", m.pendingSpawn.itemKey))
	}

	for i, name := range m.configuredRepos {
		if i == m.repoPickerCursor {
			b.WriteString(selectedItemStyle.Render("> "+name) + "\n")
		} else {
			b.WriteString("  " + name + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("  enter: select | esc: cancel"))

	rpv := tea.NewView(b.String())
	rpv.AltScreen = true
	return rpv
}

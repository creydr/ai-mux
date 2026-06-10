package dashboard

import tea "charm.land/bubbletea/v2"

func (m Model) handleRenameKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEnter:
		name := m.renameInput
		sessionID := m.renamingSession
		m.renameActive = false
		m.renamingSession = ""
		m.renameInput = ""
		if m.conn != nil {
			return m, renameSessionCmd(m.conn, sessionID, name)
		}
		return m, nil
	case msg.Code == tea.KeyEscape:
		m.renameActive = false
		m.renamingSession = ""
		m.renameInput = ""
		return m, nil
	case msg.Code == tea.KeyBackspace:
		if len(m.renameInput) > 0 {
			m.renameInput = m.renameInput[:len(m.renameInput)-1]
		}
		return m, nil
	default:
		if msg.Text != "" {
			m.renameInput += msg.Text
		}
		return m, nil
	}
}

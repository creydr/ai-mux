package dashboard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleAttachKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape:
		m.view = viewOverview
		m.attachedSession = nil
		m.attachOutput = nil
		m.rebuildViewport()
		if m.conn != nil {
			return m, detachSessionCmd(m.conn)
		}
		return m, nil
	case msg.Code == tea.KeyPgUp:
		m.attachViewport.SetYOffset(m.attachViewport.YOffset() - 10)
		return m, nil
	case msg.Code == tea.KeyPgDown:
		m.attachViewport.SetYOffset(m.attachViewport.YOffset() + 10)
		return m, nil
	case msg.Code == 'n':
		if m.attachedSession != nil {
			m.renameActive = true
			m.renamingSession = m.attachedSession.ID
			m.renameInput = m.attachedSession.Name
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderAttachView() tea.View {
	var b strings.Builder
	width := m.width
	if width == 0 {
		width = 80
	}

	header := "  Session: "
	if m.attachedSession != nil {
		badge := sessionBadge(m.attachedSession.Status, m.attachedSession.WaitingInput)
		header += fmt.Sprintf("%s  %s#%d  %s  %s",
			m.attachedSession.ID,
			m.attachedSession.Repo,
			m.attachedSession.Number,
			m.attachedSession.Agent,
			badge,
		)
	}
	b.WriteString(attachHeaderStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(attachSeparatorStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	b.WriteString(m.attachViewport.View())

	b.WriteString("\n")
	b.WriteString(attachSeparatorStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	if m.renameActive {
		b.WriteString(statusBarStyle.Render(fmt.Sprintf("  Rename: %s█  (enter: confirm | esc: cancel)", m.renameInput)))
	} else {
		b.WriteString(statusBarStyle.Render("  esc: back | n: rename | pgup/pgdn: scroll"))
	}

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *Model) rebuildAttachViewport() {
	width := m.width
	if width == 0 {
		width = 80
	}
	vpHeight := m.height - 4
	if vpHeight < 5 {
		vpHeight = 20
	}

	m.attachViewport.SetWidth(width)
	m.attachViewport.SetHeight(vpHeight)

	var lines []string
	for _, chunk := range m.attachOutput {
		chunkLines := strings.Split(chunk, "\n")
		lines = append(lines, chunkLines...)
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{statusBarStyle.Render("  Waiting for output...")}
	}

	m.attachViewport.SetContentLines(lines)
}

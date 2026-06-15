package dashboard

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape, 'q', '?':
		m.view = viewOverview
		m.rebuildViewport()
		return m, nil
	}
	return m, nil
}

func (m Model) renderHelp() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Keyboard Shortcuts"))
	b.WriteString("\n\n")

	section := func(title string, bindings [][2]string) {
		b.WriteString("  ")
		b.WriteString(repoHeaderInlineStyle.Render(title))
		b.WriteString("\n")
		for _, bind := range bindings {
			b.WriteString("    ")
			b.WriteString(selectedItemStyle.Render(" " + bind[0] + " "))
			b.WriteString("  ")
			b.WriteString(normalItemStyle.Render(bind[1]))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	section("Navigation", [][2]string{
		{"j / ↓", "Move down"},
		{"k / ↑", "Move up"},
		{"h / ←", "Focus repo panel"},
		{"l / →", "Focus item panel"},
		{"Tab", "Switch tab"},
		{"Enter", "Select / expand"},
		{":", "Search / filter"},
	})

	section("Issues & Pull Requests", [][2]string{
		{"a", "Spawn agent"},
		{"t", "Attach to session"},
		{"b / o", "Open in browser"},
		{"s", "Stop session"},
		{"r", "Refresh"},
	})

	section("Jira", [][2]string{
		{"a", "Spawn agent (select repo)"},
		{"t", "Attach to session"},
		{"b / o", "Open in browser"},
		{"s", "Stop session"},
		{"Enter", "View details / load more"},
		{"r", "Refresh"},
	})

	section("Sessions", [][2]string{
		{"Enter", "Attach to session"},
		{"n", "Rename session"},
		{"s", "Stop session"},
		{"x", "Remove session"},
	})

	section("Attached View", [][2]string{
		{"Esc", "Detach / go back"},
		{"n", "Rename session"},
		{"PgUp / PgDn", "Scroll output"},
	})

	b.WriteString(statusBarStyle.Render("  esc / q / ? : close"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

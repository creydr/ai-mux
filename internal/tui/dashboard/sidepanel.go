package dashboard

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const panelWidth = 22

func renderSidePanel(repos []string, repoCursor int, selectedRepo string, focused bool, height int) string {
	borderColor := lipgloss.Color("#555555")
	if focused {
		borderColor = lipgloss.Color("#61AFEF")
	}

	style := lipgloss.NewStyle().
		Width(panelWidth).
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderRightForeground(borderColor)

	var b strings.Builder

	label := "All"
	marker := " "
	if selectedRepo == "" {
		marker = "▸"
	}
	if repoCursor == 0 {
		b.WriteString(selectedItemStyle.Render(marker + " " + label))
	} else {
		b.WriteString(normalItemStyle.Render(marker + " " + label))
	}
	b.WriteString("\n")

	maxName := panelWidth - 4
	for i, repo := range repos {
		name := repo
		if len(name) > maxName {
			name = name[:maxName-1] + "…"
		}
		marker := " "
		if repo == selectedRepo {
			marker = "▸"
		}
		line := marker + " " + name
		if repoCursor == i+1 {
			b.WriteString(selectedItemStyle.Render(line))
		} else {
			b.WriteString(normalItemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	for i := len(repos) + 1; i < height; i++ {
		b.WriteString("\n")
	}

	return style.Render(b.String())
}

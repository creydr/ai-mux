package attach

import (
	"charm.land/lipgloss/v2"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			PaddingBottom(1)

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#AAAAAA"))

	bodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")).
			PaddingLeft(2)

	reviewStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingTop(1)

	reviewAuthorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FF79C6"))

	reviewStateStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50FA7B"))

	commentStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	commentAuthorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#8BE9FD"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))
)

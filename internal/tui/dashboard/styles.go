package dashboard

import (
	"charm.land/lipgloss/v2"
)

var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#61AFEF")).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			PaddingBottom(1)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#3C3C3C")).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC"))

	repoHeaderInlineStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#61AFEF"))

	badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#61AFEF"))

	sessionRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50FA7B")).
				Bold(true)

	sessionWaitingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F1FA8C")).
				Bold(true)

	sessionDoneStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6272A4"))

	sessionFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF5555")).
				Bold(true)

	sessionStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#50FA7B")).
				Bold(true)

	attachHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#61AFEF"))

	attachSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#444444"))
)

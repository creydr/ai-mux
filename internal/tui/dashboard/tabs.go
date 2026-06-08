package dashboard

import "strings"

type tab int

const (
	tabIssues tab = iota
	tabPRs
	tabSessions
)

var tabNames = []string{"Issues", "Pull Requests", "Sessions"}

func renderTabs(active tab, issueBadge, prBadge, sessionBadge int) string {
	var tabs []string
	badges := []int{issueBadge, prBadge, sessionBadge}

	for i, name := range tabNames {
		label := name
		if badges[i] > 0 {
			label += badgeStyle.Render(" ●")
		}

		if tab(i) == active {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}

	return tabBarStyle.Render(strings.Join(tabs, " "))
}

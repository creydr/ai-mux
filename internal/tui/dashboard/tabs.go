package dashboard

import "strings"

type tab int

const (
	tabIssues tab = iota
	tabPRs
	tabJira
	tabSessions
)

func enabledTabs(jiraEnabled bool) []tab {
	tabs := []tab{tabIssues, tabPRs}
	if jiraEnabled {
		tabs = append(tabs, tabJira)
	}
	tabs = append(tabs, tabSessions)
	return tabs
}

func tabName(t tab) string {
	switch t {
	case tabIssues:
		return "Issues"
	case tabPRs:
		return "Pull Requests"
	case tabJira:
		return "Jira"
	case tabSessions:
		return "Sessions"
	default:
		return ""
	}
}

func renderTabs(active tab, jiraEnabled bool, issueBadge, prBadge, jiraBadge, sessionBadge int) string {
	tabs := enabledTabs(jiraEnabled)
	badges := map[tab]int{
		tabIssues:   issueBadge,
		tabPRs:      prBadge,
		tabJira:     jiraBadge,
		tabSessions: sessionBadge,
	}

	var parts []string
	for _, t := range tabs {
		label := tabName(t)
		if badges[t] > 0 {
			label += badgeStyle.Render(" ●")
		}
		if t == active {
			parts = append(parts, activeTabStyle.Render(label))
		} else {
			parts = append(parts, inactiveTabStyle.Render(label))
		}
	}
	return tabBarStyle.Render(strings.Join(parts, " "))
}

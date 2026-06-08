package dashboard

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

const panelWidth = 28

type orgGroup struct {
	org   string
	repos []string
}

func groupReposByOrg(repos []string) []orgGroup {
	orgMap := make(map[string][]string)
	for _, repo := range repos {
		parts := strings.SplitN(repo, "/", 2)
		org := parts[0]
		name := repo
		if len(parts) == 2 {
			name = parts[1]
		}
		orgMap[org] = append(orgMap[org], name)
	}

	orgs := make([]string, 0, len(orgMap))
	for org := range orgMap {
		orgs = append(orgs, org)
	}
	sort.Strings(orgs)

	groups := make([]orgGroup, len(orgs))
	for i, org := range orgs {
		groups[i] = orgGroup{org: org, repos: orgMap[org]}
	}
	return groups
}

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
	lineCount := 0

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
	lineCount++

	groups := groupReposByOrg(repos)
	repoIdx := 1
	for _, g := range groups {
		b.WriteString(repoHeaderInlineStyle.Render(g.org))
		b.WriteString("\n")
		lineCount++

		for _, name := range g.repos {
			fullName := g.org + "/" + name
			maxName := panelWidth - 6
			displayName := name
			if len(displayName) > maxName {
				displayName = displayName[:maxName-1] + "…"
			}
			marker := " "
			if fullName == selectedRepo {
				marker = "▸"
			}
			line := "  " + marker + " " + displayName
			if repoCursor == repoIdx {
				b.WriteString(selectedItemStyle.Render(line))
			} else {
				b.WriteString(normalItemStyle.Render(line))
			}
			b.WriteString("\n")
			lineCount++
			repoIdx++
		}
	}

	for i := lineCount; i < height; i++ {
		b.WriteString("\n")
	}

	return style.Render(b.String())
}

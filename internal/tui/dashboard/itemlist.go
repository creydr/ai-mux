package dashboard

import (
	"fmt"
	"sort"
	"time"

	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

type visibleRow struct {
	item       *provider.Item
	expandRepo string
	allRepo    string
	isHeader   bool
	text       string
}

func buildVisibleRows(items []provider.Item, itemsPerRepo int, expanded map[string]bool, selectedRepo string, fullLoaded map[string]bool) []visibleRow {
	grouped := groupByRepo(items)
	var rows []visibleRow

	for _, g := range grouped {
		limit := len(g.items)
		if selectedRepo == "" && !expanded[g.repo] && itemsPerRepo > 0 && limit > itemsPerRepo {
			limit = itemsPerRepo
		}
		for i := 0; i < limit; i++ {
			rows = append(rows, visibleRow{item: &g.items[i]})
		}
		showExpand := false
		if selectedRepo == "" {
			if !expanded[g.repo] {
				if fullLoaded[g.repo] {
					showExpand = limit < len(g.items)
				} else {
					showExpand = len(g.items) >= itemsPerRepo
				}
			} else if !fullLoaded[g.repo] {
				showExpand = true
			}
		} else if !fullLoaded[g.repo] {
			showExpand = true
		}
		if showExpand {
			rows = append(rows, visibleRow{expandRepo: g.repo})
		}
		if !fullLoaded[g.repo] {
			rows = append(rows, visibleRow{allRepo: g.repo})
		}
	}
	return rows
}

func buildContentLines(items []provider.Item, cursor, width, itemsPerRepo int, expanded map[string]bool, selectedRepo string, fullLoaded map[string]bool, sessions []protocol.SessionPayload, loadingAll map[string]bool) ([]string, int) {
	rows := buildVisibleRows(items, itemsPerRepo, expanded, selectedRepo, fullLoaded)

	sessionMap := make(map[string]*protocol.SessionPayload)
	for i := range sessions {
		s := &sessions[i]
		key := fmt.Sprintf("%s#%d", s.Repo, s.Number)
		if s.Status == "running" || s.Status == "pending" || s.Status == "completed" {
			sessionMap[key] = s
		}
	}

	var lines []string
	cursorLine := 0
	prevRepo := ""
	rowIdx := 0

	for _, r := range rows {
		if r.item != nil {
			repo := r.item.Repo.String()
			if repo != prevRepo {
				if prevRepo != "" {
					lines = append(lines, "")
				}
				lines = append(lines, repoHeaderInlineStyle.Render("  "+repo))
				prevRepo = repo
			}
			text := formatItem(*r.item, width)
			key := fmt.Sprintf("%s#%d", r.item.Repo.String(), r.item.Number)
			if sess, ok := sessionMap[key]; ok {
				text += " " + sessionBadge(sess.Status, sess.WaitingInput)
			}
			if rowIdx == cursor {
				cursorLine = len(lines)
				lines = append(lines, selectedItemStyle.Width(width).Render(text))
			} else {
				lines = append(lines, normalItemStyle.Render(text))
			}
			rowIdx++
		} else if r.expandRepo != "" {
			var label string
			if fullLoaded[r.expandRepo] {
				remaining := countRepoItems(items, r.expandRepo) - itemsPerRepo
				label = fmt.Sprintf("    [+%d more]", remaining)
			} else {
				label = "    [more...]"
			}
			if rowIdx == cursor {
				cursorLine = len(lines)
				lines = append(lines, selectedItemStyle.Width(width).Render(label))
			} else {
				lines = append(lines, normalItemStyle.Render(label))
			}
			rowIdx++
		} else if r.allRepo != "" {
			var label string
			if loadingAll[r.allRepo] {
				label = "    [Loading...]"
			} else {
				label = "    [All...]"
			}
			if rowIdx == cursor {
				cursorLine = len(lines)
				lines = append(lines, selectedItemStyle.Width(width).Render(label))
			} else {
				lines = append(lines, normalItemStyle.Render(label))
			}
			rowIdx++
		}
	}

	return lines, cursorLine
}

func sessionBadge(status string, waitingInput bool) string {
	switch status {
	case "running", "pending":
		if waitingInput {
			return sessionWaitingStyle.Render("[waiting]")
		}
		return sessionRunningStyle.Render("[running]")
	case "completed":
		return sessionDoneStyle.Render("[done]")
	case "failed":
		return sessionFailedStyle.Render("[failed]")
	case "stopped":
		return sessionDoneStyle.Render("[stopped]")
	default:
		return ""
	}
}

func buildSessionLines(sessions []protocol.SessionPayload, cursor, width, scrollOffset int) ([]string, int) {
	if len(sessions) == 0 {
		return []string{statusBarStyle.Render("  No sessions")}, 0
	}

	const agentCol = 10
	const elapsedCol = 12

	itemCol := 0
	for _, s := range sessions {
		if s.Repo != "" && s.Number > 0 {
			l := len(fmt.Sprintf("%s#%d", s.Repo, s.Number))
			if l > itemCol {
				itemCol = l
			}
		}
	}
	if itemCol < 10 {
		itemCol = 10
	}

	// "  " + label + "  " + item + "  " + agent + "  " + badge(~10) + "  " + elapsed
	fixedCols := 2 + 2 + itemCol + 2 + agentCol + 2 + 10 + 2 + elapsedCol
	labelWidth := width - fixedCols
	if labelWidth < 16 {
		labelWidth = 16
	}

	var lines []string
	cursorLine := 0

	for i, s := range sessions {
		badge := sessionBadge(s.Status, s.WaitingInput)
		elapsed := ""
		if t, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
			elapsed = time.Since(t).Truncate(time.Second).String()
		}
		if len(elapsed) > elapsedCol {
			elapsed = elapsed[:elapsedCol]
		}

		item := ""
		if s.Repo != "" && s.Number > 0 {
			item = fmt.Sprintf("%s#%d", s.Repo, s.Number)
		}

		agent := s.Agent
		if len(agent) > agentCol {
			agent = agent[:agentCol-1] + "…"
		}

		label := s.ID
		if s.Name != "" {
			label = s.Name
		}
		if i == cursor {
			label = scrollLabel(label, labelWidth, scrollOffset)
		} else if len(label) > labelWidth {
			label = label[:labelWidth-1] + "…"
		}
		text := fmt.Sprintf("  %-*s  %-*s  %-*s %s  %-*s", labelWidth, label, itemCol, item, agentCol, agent, badge, elapsedCol, elapsed)
		if i == cursor {
			cursorLine = len(lines)
			lines = append(lines, selectedItemStyle.Width(width).Render(text))
		} else {
			lines = append(lines, normalItemStyle.Render(text))
		}
	}

	return lines, cursorLine
}

func scrollLabel(name string, maxWidth, offset int) string {
	if len(name) <= maxWidth {
		return name
	}
	maxScroll := len(name) - maxWidth
	pause := 1
	cycle := pause + maxScroll + pause
	pos := offset % cycle
	var scrollPos int
	switch {
	case pos < pause:
		scrollPos = 0
	case pos < pause+maxScroll:
		scrollPos = pos - pause
	default:
		scrollPos = maxScroll
	}
	return name[scrollPos : scrollPos+maxWidth]
}

func formatItem(item provider.Item, width int) string {
	prefix := fmt.Sprintf("  #%-5d ", item.Number)
	maxTitle := width - len(prefix) - 4
	title := item.Title
	if maxTitle > 0 && len(title) > maxTitle {
		title = title[:maxTitle-1] + "…"
	}
	return prefix + title
}

type repoGroup struct {
	repo  string
	items []provider.Item
}

func groupByRepo(items []provider.Item) []repoGroup {
	m := make(map[string][]provider.Item)
	var order []string
	for _, item := range items {
		repo := item.Repo.String()
		if _, ok := m[repo]; !ok {
			order = append(order, repo)
		}
		m[repo] = append(m[repo], item)
	}
	sort.Strings(order)

	groups := make([]repoGroup, len(order))
	for i, repo := range order {
		groups[i] = repoGroup{repo: repo, items: m[repo]}
	}
	return groups
}

func countRepoItems(items []provider.Item, repo string) int {
	count := 0
	for _, item := range items {
		if item.Repo.String() == repo {
			count++
		}
	}
	return count
}

func buildJiraContentLines(items []provider.JiraItem, cursor, width int, hasMore bool, sessions []protocol.SessionPayload, total int, loadingAllJira bool) ([]string, int) {
	if len(items) == 0 {
		return []string{statusBarStyle.Render("  No Jira items")}, 0
	}

	sessionMap := make(map[string]*protocol.SessionPayload)
	for i := range sessions {
		s := &sessions[i]
		if s.ItemKey != "" && (s.Status == "running" || s.Status == "pending" || s.Status == "completed") {
			sessionMap[s.ItemKey] = s
		}
	}

	const keyCol = 12
	const typeCol = 12
	const statusCol = 16
	const priorityCol = 10
	fixedCols := 2 + keyCol + 2 + typeCol + 2 + statusCol + 2 + priorityCol + 2
	summaryCol := width - fixedCols
	if summaryCol < 20 {
		summaryCol = 20
	}

	var lines []string
	cursorLine := 0

	for i, item := range items {
		summary := item.Summary
		if len(summary) > summaryCol {
			summary = summary[:summaryCol-1] + "…"
		}

		itemType := item.Type
		if len(itemType) > typeCol-2 {
			itemType = itemType[:typeCol-3] + "…"
		}
		typeStr := fmt.Sprintf("[%s]", itemType)

		status := item.Status
		if len(status) > statusCol-2 {
			status = status[:statusCol-3] + "…"
		}
		statusStr := fmt.Sprintf("[%s]", status)

		priority := item.Priority
		if len(priority) > priorityCol-2 {
			priority = priority[:priorityCol-3] + "…"
		}
		priorityStr := fmt.Sprintf("[%s]", priority)

		text := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %s",
			keyCol, item.Key,
			typeCol, typeStr,
			summaryCol, summary,
			statusCol, statusStr,
			priorityStr,
		)

		if sess, ok := sessionMap[item.Key]; ok {
			text += " " + sessionBadge(sess.Status, sess.WaitingInput)
		}

		if i == cursor {
			cursorLine = len(lines)
			lines = append(lines, selectedItemStyle.Width(width).Render(text))
		} else {
			lines = append(lines, normalItemStyle.Render(text))
		}
	}

	if hasMore {
		label := "    [more...]"
		idx := len(items)
		if idx == cursor {
			cursorLine = len(lines)
			lines = append(lines, selectedItemStyle.Width(width).Render(label))
		} else {
			lines = append(lines, normalItemStyle.Render(label))
		}

		var allLabel string
		if loadingAllJira {
			allLabel = fmt.Sprintf("    [Loading... (%d/%d)]", len(items), total)
		} else {
			allLabel = fmt.Sprintf("    [All... (%d items)]", total)
		}
		allIdx := idx + 1
		if allIdx == cursor {
			cursorLine = len(lines)
			lines = append(lines, selectedItemStyle.Width(width).Render(allLabel))
		} else {
			lines = append(lines, normalItemStyle.Render(allLabel))
		}
	}

	return lines, cursorLine
}

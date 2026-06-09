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
		}
		if showExpand {
			rows = append(rows, visibleRow{expandRepo: g.repo})
		}
	}
	return rows
}

func buildContentLines(items []provider.Item, cursor, width, itemsPerRepo int, expanded map[string]bool, selectedRepo string, fullLoaded map[string]bool, sessions []protocol.SessionPayload) ([]string, int) {
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

func buildSessionLines(sessions []protocol.SessionPayload, cursor, width int) ([]string, int) {
	if len(sessions) == 0 {
		return []string{statusBarStyle.Render("  No sessions")}, 0
	}

	var lines []string
	cursorLine := 0

	for i, s := range sessions {
		badge := sessionBadge(s.Status, s.WaitingInput)
		elapsed := ""
		if t, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
			elapsed = time.Since(t).Truncate(time.Second).String()
		}

		item := ""
		if s.Repo != "" && s.Number > 0 {
			item = fmt.Sprintf("%s#%d", s.Repo, s.Number)
		}
		label := s.ID
		if s.Name != "" {
			label = fmt.Sprintf("%s (%s)", s.Name, s.ID)
		}
		text := fmt.Sprintf("  %-24s  %-30s  %-10s %s  %s", label, item, s.Agent, badge, elapsed)
		if i == cursor {
			cursorLine = len(lines)
			lines = append(lines, selectedItemStyle.Width(width).Render(text))
		} else {
			lines = append(lines, normalItemStyle.Render(text))
		}
	}

	return lines, cursorLine
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

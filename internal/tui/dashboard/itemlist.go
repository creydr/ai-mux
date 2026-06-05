package dashboard

import (
	"fmt"
	"sort"
	"strings"

	"github.com/creydr/ai-mux/internal/provider"
)

func renderItemList(items []provider.Item, cursor int, width int) string {
	if len(items) == 0 {
		return statusBarStyle.Render("  No items")
	}

	grouped := groupByRepo(items)

	var b strings.Builder
	row := 0
	for _, g := range grouped {
		b.WriteString(repoHeaderStyle.Render("  " + g.repo))
		b.WriteString("\n")
		for _, item := range g.items {
			line := formatItem(item, width)
			if row == cursor {
				b.WriteString(selectedItemStyle.Render(line))
			} else {
				b.WriteString(normalItemStyle.Render(line))
			}
			b.WriteString("\n")
			row++
		}
	}
	return b.String()
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

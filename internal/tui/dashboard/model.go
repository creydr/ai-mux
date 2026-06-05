package dashboard

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

const headerLines = 6
const expandChunkSize = 20

type panel int

const (
	panelRepos panel = iota
	panelItems
)

type Model struct {
	conn   protocol.Conn
	width  int
	height int

	activeTab  tab
	cursor     int
	cursorLine int
	viewport   viewport.Model
	issues     []provider.Item
	prs        []provider.Item

	repos        []string
	repoCursor   int
	selectedRepo string
	focusPanel   panel
	expanded     map[string]bool
	fullLoaded   map[string]bool
	itemsPerRepo int

	loading    bool
	issueBadge int
	prBadge    int
	err        error
}

func New(conn protocol.Conn, itemsPerRepo int) Model {
	if itemsPerRepo <= 0 {
		itemsPerRepo = 3
	}
	vp := viewport.New()
	vp.KeyMap = viewport.KeyMap{}
	vp.FillHeight = true
	vp.MouseWheelEnabled = true
	return Model{
		conn:         conn,
		loading:      conn != nil,
		focusPanel:   panelItems,
		expanded:     make(map[string]bool),
		fullLoaded:   make(map[string]bool),
		itemsPerRepo: itemsPerRepo,
		viewport:     vp,
	}
}

func (m Model) Init() tea.Cmd {
	if m.conn == nil {
		return nil
	}
	return fetchItemsCmd(m.conn, m.itemsPerRepo)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildViewport()
		return m, nil
	case itemsReceivedMsg:
		m.issues = msg.issues
		m.prs = msg.prs
		m.cursor = 0
		m.loading = false
		m.updateRepoList()
		m.detectFullLoaded()
		m.rebuildViewport()
		return m, nil
	case repoExpandedMsg:
		m.mergeExpandedItems(msg)
		m.rebuildViewport()
		return m, nil
	case eventReceivedMsg:
		m.handleEvent(msg.event)
		m.updateRepoList()
		m.rebuildViewport()
		return m, listenEventsCmd(m.conn)
	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil
	case connectResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, fetchItemsCmd(m.conn, m.itemsPerRepo)
	}
	return m, nil
}

func (m Model) View() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  ai-mux"))
	b.WriteString("\n\n")
	b.WriteString(renderTabs(m.activeTab, m.issueBadge, m.prBadge))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(fmt.Sprintf("  Error: %v\n", m.err))
	}

	width := m.width
	if width == 0 {
		width = 80
	}

	vpHeight := m.viewportHeight()

	if m.loading && len(m.issues) == 0 && len(m.prs) == 0 {
		b.WriteString(statusBarStyle.Render("  Loading..."))
	} else {
		panelStr := renderSidePanel(m.repos, m.repoCursor, m.selectedRepo, m.focusPanel == panelRepos, vpHeight)

		items := m.currentItems()
		var listStr string
		if len(items) == 0 {
			listStr = statusBarStyle.Render("  No items")
		} else {
			listStr = m.viewport.View()
		}

		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, panelStr, listStr))
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("  h/l: panel | j/k: navigate | o: open | tab: switch | r: refresh | q: quit"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m Model) viewportHeight() int {
	h := m.height - headerLines
	if h < 1 {
		h = 20
	}
	return h
}

func (m *Model) rebuildViewport() {
	vpHeight := m.viewportHeight()
	width := m.width
	if width == 0 {
		width = 80
	}
	listWidth := width - panelWidth
	if listWidth < 20 {
		listWidth = 20
	}

	m.viewport.SetWidth(listWidth)
	m.viewport.SetHeight(vpHeight)

	items := m.currentItems()
	lines, cursorLine := buildContentLines(items, m.cursor, listWidth, m.itemsPerRepo, m.expanded, m.selectedRepo, m.fullLoaded)
	m.viewport.SetContentLines(lines)
	m.cursorLine = cursorLine

	if m.cursorLine < m.viewport.YOffset() {
		m.viewport.SetYOffset(m.cursorLine)
	} else if m.cursorLine >= m.viewport.YOffset()+vpHeight {
		m.viewport.SetYOffset(m.cursorLine - vpHeight + 1)
	}
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == 'h' || msg.Code == tea.KeyLeft:
		m.focusPanel = panelRepos
		return m, nil
	case msg.Code == 'l' || msg.Code == tea.KeyRight:
		m.focusPanel = panelItems
		return m, nil
	case msg.Code == tea.KeyTab:
		m.activeTab = (m.activeTab + 1) % tab(len(tabNames))
		m.cursor = 0
		m.expanded = make(map[string]bool)
		if m.activeTab == tabIssues {
			m.issueBadge = 0
		} else {
			m.prBadge = 0
		}
		m.rebuildViewport()
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		if m.focusPanel == panelRepos {
			maxIdx := len(m.repos)
			if m.repoCursor < maxIdx {
				m.repoCursor++
			}
		} else {
			items := m.visibleItems()
			if m.cursor < len(items)-1 {
				m.cursor++
			}
			m.rebuildViewport()
		}
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
		if m.focusPanel == panelRepos {
			if m.repoCursor > 0 {
				m.repoCursor--
			}
		} else {
			if m.cursor > 0 {
				m.cursor--
			}
			m.rebuildViewport()
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.focusPanel == panelRepos {
			if m.repoCursor == 0 {
				m.selectedRepo = ""
			} else {
				m.selectedRepo = m.repos[m.repoCursor-1]
			}
			m.cursor = 0
			m.expanded = make(map[string]bool)
			m.focusPanel = panelItems
			m.rebuildViewport()
			return m, nil
		}
		repo := m.expandableRepoAtCursor()
		if repo != "" {
			if !m.fullLoaded[repo] && m.conn != nil {
				itemType := provider.ItemTypeIssue
				var items []provider.Item
				if m.activeTab == tabPRs {
					itemType = provider.ItemTypePR
					items = m.prs
				} else {
					items = m.issues
				}
				currentCount := countRepoItems(items, repo)
				return m, expandRepoCmd(m.conn, repo, itemType, currentCount+expandChunkSize)
			}
			m.expanded[repo] = true
			m.rebuildViewport()
			return m, nil
		}
		item := m.selectedItem()
		if item != nil && item.URL != "" {
			return m, openBrowserCmd(item.URL)
		}
		return m, nil
	case msg.Code == 'o':
		item := m.selectedItem()
		if item != nil && item.URL != "" {
			return m, openBrowserCmd(item.URL)
		}
		return m, nil
	case msg.Code == 'r':
		if m.conn != nil {
			m.fullLoaded = make(map[string]bool)
			return m, fetchItemsCmd(m.conn, m.itemsPerRepo)
		}
		return m, nil
	case msg.Code == 'q' || msg.Code == tea.KeyEscape:
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) handleEvent(ev event.Event) {
	switch ev.Type {
	case event.TypeNewIssue:
		if ev.Item != nil {
			m.issues = append(m.issues, *ev.Item)
			if m.activeTab != tabIssues {
				m.issueBadge++
			}
		}
	case event.TypeNewPR:
		if ev.Item != nil {
			m.prs = append(m.prs, *ev.Item)
			if m.activeTab != tabPRs {
				m.prBadge++
			}
		}
	case event.TypeIssueUpdated:
		if ev.Item != nil {
			m.updateItem(m.issues, *ev.Item)
		}
	case event.TypePRUpdated:
		if ev.Item != nil {
			m.updateItem(m.prs, *ev.Item)
		}
	}
}

func (m Model) currentItems() []provider.Item {
	var all []provider.Item
	if m.activeTab == tabIssues {
		all = m.issues
	} else {
		all = m.prs
	}

	if m.selectedRepo == "" {
		return all
	}

	var filtered []provider.Item
	for _, item := range all {
		if item.Repo.String() == m.selectedRepo {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (m Model) visibleItems() []visibleRow {
	items := m.currentItems()
	return buildVisibleRows(items, m.itemsPerRepo, m.expanded, m.selectedRepo, m.fullLoaded)
}

func (m Model) selectedItem() *provider.Item {
	rows := m.visibleItems()
	if m.cursor >= 0 && m.cursor < len(rows) {
		r := rows[m.cursor]
		if r.item != nil {
			return r.item
		}
	}
	return nil
}

func (m Model) expandableRepoAtCursor() string {
	rows := m.visibleItems()
	if m.cursor >= 0 && m.cursor < len(rows) {
		return rows[m.cursor].expandRepo
	}
	return ""
}

func (m *Model) updateRepoList() {
	seen := make(map[string]bool)
	for _, item := range m.issues {
		seen[item.Repo.String()] = true
	}
	for _, item := range m.prs {
		seen[item.Repo.String()] = true
	}
	m.repos = make([]string, 0, len(seen))
	for repo := range seen {
		m.repos = append(m.repos, repo)
	}
	sort.Strings(m.repos)
}

func (m *Model) detectFullLoaded() {
	repoIssueCount := make(map[string]int)
	for _, item := range m.issues {
		repoIssueCount[item.Repo.String()]++
	}
	repoPRCount := make(map[string]int)
	for _, item := range m.prs {
		repoPRCount[item.Repo.String()]++
	}
	for _, repo := range m.repos {
		if repoIssueCount[repo] < m.itemsPerRepo && repoPRCount[repo] < m.itemsPerRepo {
			m.fullLoaded[repo] = true
		}
	}
}

func (m *Model) mergeExpandedItems(msg repoExpandedMsg) {
	if msg.itemType == provider.ItemTypeIssue {
		m.issues = replaceRepoItems(m.issues, msg.repo, msg.items)
	} else {
		m.prs = replaceRepoItems(m.prs, msg.repo, msg.items)
	}
	if msg.requestedLimit <= 0 || len(msg.items) < msg.requestedLimit {
		m.fullLoaded[msg.repo] = true
	}
	m.expanded[msg.repo] = true
	m.updateRepoList()
}

func replaceRepoItems(all []provider.Item, repo string, newItems []provider.Item) []provider.Item {
	var kept []provider.Item
	for _, item := range all {
		if item.Repo.String() != repo {
			kept = append(kept, item)
		}
	}
	return append(kept, newItems...)
}

func (m Model) updateItem(items []provider.Item, updated provider.Item) {
	for i, item := range items {
		if item.ID == updated.ID {
			items[i] = updated
			return
		}
	}
}

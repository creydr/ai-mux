package dashboard

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/tui/attach"
)

const headerLines = 6
const expandChunkSize = 20

type viewState int

const (
	viewOverview viewState = iota
	viewAttach
	viewItemDetail
	viewAgentPicker
	viewWorktreeChoice
)

type spawnRequest struct {
	repo     string
	number   int
	itemType string
	agent    string
}

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

	sessions      []protocol.SessionPayload
	sessionCursor int
	sessionBadge  int
	agents        []string
	defaultAgent  string

	agentCursor       int
	pendingSpawn      *spawnRequest
	worktreeChoiceIdx int

	statusText   string
	statusTickID int

	view            viewState
	attachedSession *protocol.SessionPayload
	attachOutput    []string
	attachViewport  viewport.Model

	itemDetail *attach.Model
}

func New(conn protocol.Conn, itemsPerRepo int, agents []string, defaultAgent string) Model {
	if itemsPerRepo <= 0 {
		itemsPerRepo = 3
	}
	vp := viewport.New()
	vp.KeyMap = viewport.KeyMap{}
	vp.FillHeight = true
	vp.MouseWheelEnabled = true
	avp := viewport.New()
	avp.KeyMap = viewport.KeyMap{}
	avp.FillHeight = true
	avp.MouseWheelEnabled = true
	return Model{
		conn:           conn,
		loading:        conn != nil,
		focusPanel:     panelItems,
		expanded:       make(map[string]bool),
		fullLoaded:     make(map[string]bool),
		itemsPerRepo:   itemsPerRepo,
		viewport:       vp,
		agents:         agents,
		defaultAgent:   defaultAgent,
		attachViewport: avp,
	}
}

func (m Model) Init() tea.Cmd {
	if m.conn == nil {
		return nil
	}
	return tea.Batch(
		fetchItemsCmd(m.conn, m.itemsPerRepo),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		var cmd tea.Cmd
		if m.view == viewAttach {
			m.attachViewport, cmd = m.attachViewport.Update(msg)
		} else {
			m.viewport, cmd = m.viewport.Update(msg)
		}
		return m, cmd
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.view == viewAttach {
			m.rebuildAttachViewport()
		} else if m.view == viewItemDetail && m.itemDetail != nil {
			updated, _ := m.itemDetail.Update(msg)
			detail := updated.(attach.Model)
			m.itemDetail = &detail
		} else {
			m.rebuildViewport()
		}
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
	case sessionsReceivedMsg:
		m.sessions = msg.sessions
		m.rebuildViewport()
		return m, nil
	case sessionSpawnedMsg:
		m.sessions = append(m.sessions, msg.session)
		return m, tmuxAttachCmd(msg.session.ID)
	case tmuxDetachedMsg:
		m.view = viewOverview
		m.rebuildViewport()
		if m.conn != nil {
			return m, fetchSessionsCmd(m.conn)
		}
		return m, nil
	case sessionStoppedMsg:
		for i, s := range m.sessions {
			if s.ID == msg.sessionID {
				m.sessions[i].Status = "stopped"
				break
			}
		}
		m.statusText = "Session stopped"
		m.rebuildViewport()
		return m, m.scheduleStatusClear()
	case statusMsg:
		m.statusText = msg.text
		return m, m.scheduleStatusClear()
	case statusTickMsg:
		if msg.id == m.statusTickID && time.Now().After(msg.due) {
			m.statusText = ""
		}
		return m, nil
	case attach.CloseMsg:
		m.view = viewOverview
		m.itemDetail = nil
		m.rebuildViewport()
		return m, nil
	case attach.SpawnSessionMsg:
		if len(m.agents) == 0 {
			m.statusText = "No agents configured"
			m.view = viewOverview
			m.itemDetail = nil
			m.rebuildViewport()
			return m, m.scheduleStatusClear()
		}
		itemType := "issue"
		if msg.Ref.Type == provider.ItemTypePR {
			itemType = "pr"
		}
		req := &spawnRequest{repo: msg.Ref.Owner + "/" + msg.Ref.Repo, number: msg.Ref.Number, itemType: itemType}
		if m.defaultAgent != "" {
			m.view = viewOverview
			m.itemDetail = nil
			m.rebuildViewport()
			return m, spawnSessionCmd(m.conn, req.repo, req.number, req.itemType, m.defaultAgent, "")
		}
		m.pendingSpawn = req
		m.agentCursor = 0
		m.view = viewAgentPicker
		m.itemDetail = nil
		return m, nil
	case worktreeExistsMsg:
		m.pendingSpawn = &spawnRequest{repo: msg.repo, number: msg.number, itemType: msg.itemType, agent: msg.agent}
		m.worktreeChoiceIdx = 0
		m.view = viewWorktreeChoice
		return m, nil
	case sessionAttachedMsg:
		m.view = viewAttach
		m.attachOutput = nil
		m.rebuildAttachViewport()
		return m, listenAttachOutputCmd(m.conn)
	case sessionOutputMsg:
		if m.view == viewAttach {
			m.attachOutput = []string{msg.data}
			m.rebuildAttachViewport()
			return m, listenAttachOutputCmd(m.conn)
		}
		return m, nil
	case attachNonOutputMsg:
		if m.view == viewAttach {
			return m, listenAttachOutputCmd(m.conn)
		}
		return m, nil
	default:
		if m.view == viewItemDetail && m.itemDetail != nil {
			updated, cmd := m.itemDetail.Update(msg)
			detail := updated.(attach.Model)
			m.itemDetail = &detail
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	if m.view == viewAttach {
		return m.renderAttachView()
	}
	if m.view == viewItemDetail && m.itemDetail != nil {
		return m.itemDetail.View()
	}
	if m.view == viewAgentPicker {
		return m.renderAgentPicker()
	}
	if m.view == viewWorktreeChoice {
		return m.renderWorktreeChoice()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  ai-mux"))
	b.WriteString("\n\n")
	b.WriteString(renderTabs(m.activeTab, m.issueBadge, m.prBadge, m.sessionBadge))
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
	} else if m.activeTab == tabSessions {
		b.WriteString(m.viewport.View())
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
	if m.statusText != "" {
		b.WriteString(sessionStatusStyle.Render("  " + m.statusText))
	} else {
		b.WriteString(statusBarStyle.Render("  " + m.statusBarText()))
	}

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

	if m.activeTab == tabSessions {
		m.viewport.SetWidth(width)
		m.viewport.SetHeight(vpHeight)
		lines, cursorLine := buildSessionLines(m.sessions, m.sessionCursor, width)
		m.viewport.SetContentLines(lines)
		m.cursorLine = cursorLine
	} else {
		listWidth := width - panelWidth
		if listWidth < 20 {
			listWidth = 20
		}
		m.viewport.SetWidth(listWidth)
		m.viewport.SetHeight(vpHeight)

		items := m.currentItems()
		lines, cursorLine := buildContentLines(items, m.cursor, listWidth, m.itemsPerRepo, m.expanded, m.selectedRepo, m.fullLoaded, m.sessions)
		m.viewport.SetContentLines(lines)
		m.cursorLine = cursorLine
	}

	if m.cursorLine < m.viewport.YOffset() {
		m.viewport.SetYOffset(m.cursorLine)
	} else if m.cursorLine >= m.viewport.YOffset()+vpHeight {
		m.viewport.SetYOffset(m.cursorLine - vpHeight + 1)
	}
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.Code == 'c' && msg.Mod.Contains(tea.ModCtrl) {
		return m, tea.Quit
	}
	if m.view == viewAttach {
		return m.handleAttachKey(msg)
	}
	if m.view == viewAgentPicker {
		return m.handleAgentPickerKey(msg)
	}
	if m.view == viewWorktreeChoice {
		return m.handleWorktreeChoiceKey(msg)
	}
	if m.view == viewItemDetail && m.itemDetail != nil {
		updated, cmd := m.itemDetail.Update(msg)
		detail := updated.(attach.Model)
		m.itemDetail = &detail
		return m, cmd
	}

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
		m.sessionCursor = 0
		m.expanded = make(map[string]bool)
		switch m.activeTab {
		case tabIssues:
			m.issueBadge = 0
		case tabPRs:
			m.prBadge = 0
		case tabSessions:
			m.sessionBadge = 0
			if m.conn != nil {
				m.rebuildViewport()
				return m, fetchSessionsCmd(m.conn)
			}
		}
		m.rebuildViewport()
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		if m.activeTab == tabSessions {
			if m.sessionCursor < len(m.sessions)-1 {
				m.sessionCursor++
			}
			m.rebuildViewport()
		} else if m.focusPanel == panelRepos {
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
		if m.activeTab == tabSessions {
			if m.sessionCursor > 0 {
				m.sessionCursor--
			}
			m.rebuildViewport()
		} else if m.focusPanel == panelRepos {
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
		if m.activeTab == tabSessions {
			if m.sessionCursor >= 0 && m.sessionCursor < len(m.sessions) {
				sess := m.sessions[m.sessionCursor]
				if sess.Status == "running" || sess.Status == "pending" {
					return m, tmuxAttachCmd(sess.ID)
				}
				m.attachedSession = &sess
				if m.conn != nil {
					return m, attachSessionCmd(m.conn, sess.ID)
				}
			}
			return m, nil
		}
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
		if item != nil {
			ref := attach.Ref{
				Type:   item.Type,
				Owner:  item.Repo.Owner,
				Repo:   item.Repo.Repo,
				Number: item.Number,
			}
			detail := attach.NewEmbedded(m.conn, ref, m.width, m.height)
			m.itemDetail = &detail
			m.view = viewItemDetail
			return m, m.itemDetail.Init()
		}
		return m, nil
	case msg.Code == 'o' || msg.Code == 'b':
		item := m.selectedItem()
		if item != nil && item.URL != "" {
			return m, openBrowserCmd(item.URL)
		}
		return m, nil
	case msg.Code == 'c':
		if m.activeTab == tabSessions {
			return m, nil
		}
		if len(m.agents) == 0 {
			m.statusText = "No agents configured"
			return m, m.scheduleStatusClear()
		}
		item := m.selectedItem()
		if item == nil {
			return m, nil
		}
		itemType := "issue"
		if m.activeTab == tabPRs {
			itemType = "pr"
		}
		req := &spawnRequest{repo: item.Repo.String(), number: item.Number, itemType: itemType}
		if m.defaultAgent != "" {
			return m, spawnSessionCmd(m.conn, req.repo, req.number, req.itemType, m.defaultAgent, "")
		}
		m.pendingSpawn = req
		m.agentCursor = 0
		m.view = viewAgentPicker
		return m, nil
	case msg.Code == 's':
		if m.activeTab == tabSessions {
			if m.sessionCursor >= 0 && m.sessionCursor < len(m.sessions) {
				sess := m.sessions[m.sessionCursor]
				if sess.Status == "running" || sess.Status == "pending" {
					return m, stopSessionCmd(m.conn, sess.ID)
				}
			}
		} else {
			item := m.selectedItem()
			if item != nil {
				for _, sess := range m.sessions {
					if sess.Repo == item.Repo.String() && sess.Number == item.Number && (sess.Status == "running" || sess.Status == "pending") {
						return m, stopSessionCmd(m.conn, sess.ID)
					}
				}
			}
		}
		return m, nil
	case msg.Code == 'r':
		if m.conn != nil {
			m.fullLoaded = make(map[string]bool)
			return m, fetchItemsCmd(m.conn, m.itemsPerRepo)
		}
		return m, nil
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

func (m *Model) scheduleStatusClear() tea.Cmd {
	m.statusTickID++
	id := m.statusTickID
	due := time.Now().Add(5 * time.Second)
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return statusTickMsg{id: id, due: due}
	})
}

func (m Model) statusBarText() string {
	switch m.activeTab {
	case tabSessions:
		return "enter: attach (ctrl-b d: detach) | s: stop | tab: switch | ctrl-c: quit"
	default:
		return "c: agent | b: browser | tab: switch | r: refresh | ctrl-c: quit"
	}
}

func (m Model) sessionForItem(repo string, number int) *protocol.SessionPayload {
	for i := range m.sessions {
		s := &m.sessions[i]
		if s.Repo == repo && s.Number == number {
			return s
		}
	}
	return nil
}

func (m Model) handleAttachKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape:
		m.view = viewOverview
		m.attachedSession = nil
		m.attachOutput = nil
		m.rebuildViewport()
		if m.conn != nil {
			return m, detachSessionCmd(m.conn)
		}
		return m, nil
	case msg.Code == tea.KeyPgUp:
		m.attachViewport.SetYOffset(m.attachViewport.YOffset() - 10)
		return m, nil
	case msg.Code == tea.KeyPgDown:
		m.attachViewport.SetYOffset(m.attachViewport.YOffset() + 10)
		return m, nil
	}
	return m, nil
}

func (m Model) renderAttachView() tea.View {
	var b strings.Builder
	width := m.width
	if width == 0 {
		width = 80
	}

	header := "  Session: "
	if m.attachedSession != nil {
		badge := sessionBadge(m.attachedSession.Status, m.attachedSession.WaitingInput)
		header += fmt.Sprintf("%s  %s#%d  %s  %s",
			m.attachedSession.ID,
			m.attachedSession.Repo,
			m.attachedSession.Number,
			m.attachedSession.Agent,
			badge,
		)
	}
	b.WriteString(attachHeaderStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(attachSeparatorStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	b.WriteString(m.attachViewport.View())

	b.WriteString("\n")
	b.WriteString(attachSeparatorStyle.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	helpText := "esc: back | pgup/pgdn: scroll"
	b.WriteString(statusBarStyle.Render("  " + helpText))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *Model) rebuildAttachViewport() {
	width := m.width
	if width == 0 {
		width = 80
	}
	vpHeight := m.height - 4
	if vpHeight < 5 {
		vpHeight = 20
	}

	m.attachViewport.SetWidth(width)
	m.attachViewport.SetHeight(vpHeight)

	var lines []string
	for _, chunk := range m.attachOutput {
		chunkLines := strings.Split(chunk, "\n")
		lines = append(lines, chunkLines...)
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{statusBarStyle.Render("  Waiting for output...")}
	}

	m.attachViewport.SetContentLines(lines)
}

func (m Model) handleAgentPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape || msg.Code == 'q':
		m.view = viewOverview
		m.pendingSpawn = nil
		m.rebuildViewport()
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		if m.agentCursor < len(m.agents)-1 {
			m.agentCursor++
		}
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
		if m.agentCursor > 0 {
			m.agentCursor--
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.pendingSpawn != nil && m.agentCursor < len(m.agents) && m.conn != nil {
			agent := m.agents[m.agentCursor]
			req := m.pendingSpawn
			m.pendingSpawn = nil
			m.view = viewOverview
			m.rebuildViewport()
			return m, spawnSessionCmd(m.conn, req.repo, req.number, req.itemType, agent, "")
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderAgentPicker() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Select Agent"))
	b.WriteString("\n\n")

	if m.pendingSpawn != nil {
		b.WriteString(fmt.Sprintf("  Spawning session for %s#%d\n\n", m.pendingSpawn.repo, m.pendingSpawn.number))
	}

	for i, name := range m.agents {
		if i == m.agentCursor {
			b.WriteString(selectedItemStyle.Render("> "+name) + "\n")
		} else {
			b.WriteString("  " + name + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("  enter: select | esc: cancel"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

var worktreeChoices = []struct {
	label  string
	action string
}{
	{"Resume in existing worktree", "reuse"},
	{"Start fresh (remove old worktree)", "fresh"},
	{"Create new (keep old worktree)", "new"},
}

func (m Model) handleWorktreeChoiceKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape || msg.Code == 'q':
		m.view = viewOverview
		m.pendingSpawn = nil
		m.rebuildViewport()
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		if m.worktreeChoiceIdx < len(worktreeChoices)-1 {
			m.worktreeChoiceIdx++
		}
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
		if m.worktreeChoiceIdx > 0 {
			m.worktreeChoiceIdx--
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.pendingSpawn != nil && m.conn != nil {
			req := m.pendingSpawn
			action := worktreeChoices[m.worktreeChoiceIdx].action
			m.pendingSpawn = nil
			m.view = viewOverview
			m.rebuildViewport()
			return m, spawnSessionCmd(m.conn, req.repo, req.number, req.itemType, req.agent, action)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderWorktreeChoice() tea.View {
	var b strings.Builder

	b.WriteString(titleStyle.Render("  Worktree Already Exists"))
	b.WriteString("\n\n")

	if m.pendingSpawn != nil {
		b.WriteString(fmt.Sprintf("  A worktree for %s#%d already exists.\n", m.pendingSpawn.repo, m.pendingSpawn.number))
		b.WriteString("  What would you like to do?\n\n")
	}

	for i, choice := range worktreeChoices {
		if i == m.worktreeChoiceIdx {
			b.WriteString(selectedItemStyle.Render("> "+choice.label) + "\n")
		} else {
			b.WriteString("  " + choice.label + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("  enter: select | esc: cancel"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

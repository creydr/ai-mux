package dashboard

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/tui"
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
	viewSessionPicker
	viewHelp
	viewRepoPicker
)

type spawnRequest struct {
	repo          string
	number        int
	itemType      string
	agent         string
	itemKey       string
	contextPrompt string
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

	agentCursor       int
	pendingSpawn      *spawnRequest
	worktreeChoiceIdx int

	sessionPickerItems  []protocol.SessionPayload
	sessionPickerCursor int

	statusText   string
	statusTickID int

	view            viewState
	attachedSession *protocol.SessionPayload
	attachOutput    []string
	attachViewport  viewport.Model

	itemDetail *attach.Model

	renameActive    bool
	renameInput     string
	renamingSession string

	sessionTickID    int
	sessionScrollPos int

	jiraEnabled      bool
	jiraItems        []provider.JiraItem
	jiraCursor       int
	jiraBadge        int
	jiraHasMore      bool
	jiraOffset       int
	configuredRepos  []string
	repoPickerActive bool
	repoPickerCursor int
	enabledTabs      []tab

	searchActive     bool
	searchInput      string
	searchCommitted  bool
	preCursorPos     int
	preJiraCursor    int
	preSessionCursor int

	jiraTotal       int
	loadingAllRepos map[string]bool
	loadingAllJira  bool
}

func New(conn protocol.Conn, itemsPerRepo int, agents []string, jiraEnabled bool, repoNames []string) Model {
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
	tabs := enabledTabs(jiraEnabled)
	return Model{
		conn:            conn,
		loading:         conn != nil,
		focusPanel:      panelItems,
		expanded:        make(map[string]bool),
		fullLoaded:      make(map[string]bool),
		loadingAllRepos: make(map[string]bool),
		itemsPerRepo:    itemsPerRepo,
		viewport:        vp,
		agents:          agents,
		attachViewport:  avp,
		jiraEnabled:     jiraEnabled,
		configuredRepos: repoNames,
		enabledTabs:     tabs,
	}
}

func (m Model) Init() tea.Cmd {
	if m.conn == nil {
		return nil
	}
	cmds := []tea.Cmd{fetchItemsCmd(m.conn, m.itemsPerRepo)}
	if m.jiraEnabled {
		cmds = append(cmds, fetchJiraItemsCmd(m.conn, 0, 0))
	}
	return tea.Batch(cmds...)
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
			if detail, ok := updated.(attach.Model); ok {
				m.itemDetail = &detail
			}
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
		delete(m.loadingAllRepos, msg.repo)
		m.mergeExpandedItems(msg)
		m.rebuildViewport()
		return m, nil
	case eventReceivedMsg:
		m.handleEvent(msg.event)
		m.updateRepoList()
		m.rebuildViewport()
		return m, listenEventsCmd(m.conn)
	case tui.ErrMsg:
		m.err = msg.Err
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
		return m, tmuxAttachCmd(msg.session.ID, msg.session.Name)
	case tmuxDetachedMsg:
		m.view = viewOverview
		m.rebuildViewport()
		if m.conn != nil {
			cmds := []tea.Cmd{fetchSessionsCmd(m.conn)}
			if m.activeTab == tabSessions {
				cmds = append(cmds, m.startSessionTick())
			}
			return m, tea.Batch(cmds...)
		}
		return m, nil
	case sessionRenamedMsg:
		for i, s := range m.sessions {
			if s.ID == msg.sessionID {
				m.sessions[i].Name = msg.name
				break
			}
		}
		if m.attachedSession != nil && m.attachedSession.ID == msg.sessionID {
			m.attachedSession.Name = msg.name
		}
		m.statusText = "Session renamed"
		m.rebuildViewport()
		return m, m.scheduleStatusClear()
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
	case sessionRemovedMsg:
		for i, s := range m.sessions {
			if s.ID == msg.sessionID {
				m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
				if m.sessionCursor >= len(m.sessions) && m.sessionCursor > 0 {
					m.sessionCursor--
				}
				break
			}
		}
		m.statusText = "Session removed"
		m.rebuildViewport()
		return m, m.scheduleStatusClear()
	case statusMsg:
		m.statusText = msg.text
		return m, m.scheduleStatusClear()
	case sessionTickMsg:
		if msg.id != m.sessionTickID {
			return m, nil
		}
		if m.activeTab == tabSessions && m.view == viewOverview {
			m.sessionScrollPos++
			m.rebuildViewport()
			return m, m.startSessionTick()
		}
		return m, nil
	case jiraItemsReceivedMsg:
		if msg.offset == 0 {
			m.jiraItems = msg.items
		} else {
			m.jiraItems = append(m.jiraItems, msg.items...)
		}
		m.jiraHasMore = len(m.jiraItems) < msg.total
		m.jiraOffset = msg.offset + len(msg.items)
		m.jiraTotal = msg.total
		m.rebuildViewport()
		if m.loadingAllJira && m.jiraHasMore && m.conn != nil {
			return m, fetchJiraItemsCmd(m.conn, m.jiraOffset, 0)
		}
		m.loadingAllJira = false
		return m, nil
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
	case attach.AttachSessionMsg:
		m.view = viewOverview
		m.itemDetail = nil
		m.rebuildViewport()
		if msg.Status == "running" || msg.Status == "pending" {
			return m, tmuxAttachCmd(msg.SessionID, msg.Name)
		}
		m.attachedSession = &protocol.SessionPayload{ID: msg.SessionID, Name: msg.Name, Status: msg.Status}
		if m.conn != nil {
			return m, attachSessionCmd(m.conn, msg.SessionID)
		}
		return m, nil
	case attach.SpawnSessionMsg:
		if len(m.agents) == 0 {
			m.statusText = "No agents configured"
			m.view = viewOverview
			m.itemDetail = nil
			m.rebuildViewport()
			return m, m.scheduleStatusClear()
		}
		spawnItem := m.findItem(msg.Ref.Owner+"/"+msg.Ref.Repo, msg.Ref.Number)
		cp := contextPromptForItem(spawnItem)
		req := &spawnRequest{repo: msg.Ref.Owner + "/" + msg.Ref.Repo, number: msg.Ref.Number, itemType: string(msg.Ref.Type), contextPrompt: cp}
		if len(m.agents) == 1 {
			m.view = viewOverview
			m.itemDetail = nil
			m.rebuildViewport()
			return m, spawnSessionCmd(m.conn, req.repo, req.number, req.itemType, m.agents[0], "", req.contextPrompt)
		}
		m.pendingSpawn = req
		m.agentCursor = 0
		m.view = viewAgentPicker
		m.itemDetail = nil
		return m, nil
	case attach.SpawnJiraSessionMsg:
		if len(m.agents) == 0 {
			m.statusText = "No agents configured"
			m.view = viewOverview
			m.itemDetail = nil
			m.rebuildViewport()
			return m, m.scheduleStatusClear()
		}
		jiraItem := m.findJiraItem(msg.Key)
		cp := contextPromptForJiraItem(jiraItem)
		m.view = viewOverview
		m.itemDetail = nil
		if len(m.configuredRepos) == 1 {
			req := &spawnRequest{repo: m.configuredRepos[0], itemType: string(provider.ItemTypeJira), itemKey: msg.Key, contextPrompt: cp}
			if len(m.agents) == 1 {
				m.rebuildViewport()
				return m, spawnJiraSessionCmd(m.conn, req.repo, req.itemKey, m.agents[0], "", req.contextPrompt)
			}
			m.pendingSpawn = req
			m.agentCursor = 0
			m.view = viewAgentPicker
			return m, nil
		}
		m.pendingSpawn = &spawnRequest{itemType: string(provider.ItemTypeJira), itemKey: msg.Key, contextPrompt: cp}
		m.repoPickerCursor = 0
		m.repoPickerActive = true
		m.view = viewRepoPicker
		return m, nil
	case worktreeExistsMsg:
		m.pendingSpawn = &spawnRequest{repo: msg.repo, number: msg.number, itemType: msg.itemType, agent: msg.agent, itemKey: msg.itemKey}
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
			if detail, ok := updated.(attach.Model); ok {
				m.itemDetail = &detail
			}
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
	if m.view == viewSessionPicker {
		return m.renderSessionPicker()
	}
	if m.view == viewRepoPicker {
		return m.renderRepoPicker()
	}
	if m.view == viewHelp {
		return m.renderHelp()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  ai-mux"))
	b.WriteString("\n\n")
	b.WriteString(renderTabs(m.activeTab, m.jiraEnabled, m.issueBadge, m.prBadge, m.jiraBadge, m.sessionBadge))
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
		if m.renameActive {
			b.WriteString(m.viewport.View())
			b.WriteString("\n")
			b.WriteString(statusBarStyle.Render(fmt.Sprintf("  Rename: %s█", m.renameInput)))
		} else {
			b.WriteString(m.viewport.View())
		}
	} else if m.activeTab == tabJira {
		if len(m.jiraItems) == 0 {
			b.WriteString(statusBarStyle.Render("  No Jira items"))
		} else {
			b.WriteString(m.viewport.View())
		}
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

	searching := m.searchActive || m.searchCommitted

	if m.activeTab == tabJira {
		m.viewport.SetWidth(width)
		m.viewport.SetHeight(vpHeight)
		jiraItems := m.filteredJiraItems()
		showMore := m.jiraHasMore && !searching
		maxCursor := len(jiraItems) - 1
		if showMore {
			maxCursor += 2
		}
		if m.jiraCursor > maxCursor {
			m.jiraCursor = maxCursor
		}
		if m.jiraCursor < 0 {
			m.jiraCursor = 0
		}
		lines, cursorLine := buildJiraContentLines(jiraItems, m.jiraCursor, width, showMore, m.sessions, m.jiraTotal, m.loadingAllJira)
		if m.searchActive {
			searchLine := searchBarStyle.Render("> " + m.searchInput + "█")
			lines = append([]string{searchLine}, lines...)
			cursorLine++
		}
		m.viewport.SetContentLines(lines)
		m.cursorLine = cursorLine
	} else if m.activeTab == tabSessions {
		m.viewport.SetWidth(width)
		m.viewport.SetHeight(vpHeight)
		sessions := m.filteredSessions()
		lines, cursorLine := buildSessionLines(sessions, m.sessionCursor, width, m.sessionScrollPos)
		if m.searchActive {
			searchLine := searchBarStyle.Render("> " + m.searchInput + "█")
			lines = append([]string{searchLine}, lines...)
			cursorLine++
		}
		m.viewport.SetContentLines(lines)
		m.cursorLine = cursorLine
	} else {
		listWidth := width - panelWidth
		if listWidth < 20 {
			listWidth = 20
		}
		m.viewport.SetWidth(listWidth)
		m.viewport.SetHeight(vpHeight)

		items := m.filteredItems()
		lines, cursorLine := buildContentLines(items, m.cursor, listWidth, m.itemsPerRepo, m.expanded, m.selectedRepo, m.fullLoaded, m.sessions, m.loadingAllRepos)
		if m.searchActive {
			searchLine := searchBarStyle.Render("> " + m.searchInput + "█")
			lines = append([]string{searchLine}, lines...)
			cursorLine++
		}
		m.viewport.SetContentLines(lines)
		m.cursorLine = cursorLine
	}

	if m.cursorLine < m.viewport.YOffset() {
		m.viewport.SetYOffset(m.cursorLine)
	} else if m.cursorLine >= m.viewport.YOffset()+vpHeight {
		m.viewport.SetYOffset(m.cursorLine - vpHeight + 1)
	}
}

func (m Model) findItem(repo string, number int) *provider.Item {
	for i := range m.issues {
		if m.issues[i].Repo.String() == repo && m.issues[i].Number == number {
			return &m.issues[i]
		}
	}
	for i := range m.prs {
		if m.prs[i].Repo.String() == repo && m.prs[i].Number == number {
			return &m.prs[i]
		}
	}
	return nil
}

func (m Model) findJiraItem(key string) *provider.JiraItem {
	for i := range m.jiraItems {
		if m.jiraItems[i].Key == key {
			return &m.jiraItems[i]
		}
	}
	return nil
}

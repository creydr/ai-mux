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
		req := &spawnRequest{repo: msg.Ref.Owner + "/" + msg.Ref.Repo, number: msg.Ref.Number, itemType: string(msg.Ref.Type)}
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
	if m.view == viewSessionPicker {
		return m.renderSessionPicker()
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
		if m.renameActive {
			b.WriteString(m.viewport.View())
			b.WriteString("\n")
			b.WriteString(statusBarStyle.Render(fmt.Sprintf("  Rename: %s█", m.renameInput)))
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

	if m.activeTab == tabSessions {
		m.viewport.SetWidth(width)
		m.viewport.SetHeight(vpHeight)
		lines, cursorLine := buildSessionLines(m.sessions, m.sessionCursor, width, m.sessionScrollPos)
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

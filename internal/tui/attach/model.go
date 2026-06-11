package attach

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	browser "github.com/creydr/ai-mux/internal/action/browser"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/tui"
)

type jiraDetailState struct {
	item         *provider.JiraItem
	comments     []provider.JiraComment
	key          string
	scroll       int
	childCursor  int
	childFocused bool
}

type Model struct {
	conn     protocol.Conn
	ref      Ref
	width    int
	height   int
	embedded bool

	item     *provider.Item
	reviews  []provider.Review
	comments []provider.Comment
	scroll   int
	err      error

	jiraKey      string
	jiraItem     *provider.JiraItem
	jiraComments []provider.JiraComment

	childCursor  int
	childFocused bool
	parentStack  []jiraDetailState

	renderedLines []string

	sessions      []protocol.SessionPayload
	sessionPicker bool
	sessionCursor int
	statusText    string
}

func New(conn protocol.Conn, ref Ref) Model {
	return Model{
		conn: conn,
		ref:  ref,
	}
}

func NewEmbedded(conn protocol.Conn, ref Ref, width, height int, item *provider.Item) Model {
	return Model{
		conn:     conn,
		ref:      ref,
		embedded: true,
		width:    width,
		height:   height,
		item:     item,
	}
}

func NewEmbeddedJira(conn protocol.Conn, key string, width, height int, item *provider.JiraItem) Model {
	return Model{
		conn:     conn,
		ref:      Ref{Type: provider.ItemTypeJira, Key: key},
		embedded: true,
		width:    width,
		height:   height,
		jiraKey:  key,
		jiraItem: item,
	}
}

func (m Model) Init() tea.Cmd {
	if m.conn == nil && m.item == nil && m.jiraItem == nil {
		return nil
	}
	if m.jiraKey != "" && m.conn != nil {
		return fetchJiraItemDetailCmd(m.conn, m.jiraKey)
	}
	if m.item != nil {
		return renderContentCmd(m.item, m.reviews, m.comments, m.width, m.err)
	}
	return fetchItemCmd(m.conn, m.ref)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.jiraItem != nil {
			return m, renderJiraContentCmd(m.jiraItem, m.jiraComments, m.width, m.childCursor, m.childFocused, m.err)
		}
		return m, renderContentCmd(m.item, m.reviews, m.comments, m.width, m.err)
	case contentRenderedMsg:
		m.renderedLines = msg.lines
		return m, nil
	case itemLoadedMsg:
		m.item = msg.item
		return m, renderContentCmd(m.item, m.reviews, m.comments, m.width, m.err)
	case reviewsLoadedMsg:
		m.reviews = msg.reviews
		return m, renderContentCmd(m.item, m.reviews, m.comments, m.width, m.err)
	case commentsLoadedMsg:
		m.comments = msg.comments
		return m, renderContentCmd(m.item, m.reviews, m.comments, m.width, m.err)
	case jiraItemLoadedMsg:
		m.jiraItem = msg.item
		m.jiraComments = msg.comments
		return m, renderJiraContentCmd(m.jiraItem, m.jiraComments, m.width, m.childCursor, m.childFocused, m.err)
	case tui.ErrMsg:
		m.err = msg.Err
		return m, renderContentCmd(m.item, m.reviews, m.comments, m.width, m.err)
	case sessionsLoadedMsg:
		if len(msg.sessions) == 0 {
			m.statusText = "No sessions for this item"
			return m, nil
		}
		if len(msg.sessions) == 1 {
			return m, func() tea.Msg {
				return AttachSessionMsg{SessionID: msg.sessions[0].ID, Name: msg.sessions[0].Name, Status: msg.sessions[0].Status}
			}
		}
		m.sessions = msg.sessions
		m.sessionCursor = 0
		m.sessionPicker = true
		return m, nil
	case statusTextMsg:
		m.statusText = msg.text
		return m, nil
	}
	return m, nil
}

func renderContentCmd(item *provider.Item, reviews []provider.Review, comments []provider.Comment, width int, err error) tea.Cmd {
	return func() tea.Msg {
		var b strings.Builder
		b.WriteString(renderHeader(item))
		b.WriteString("\n\n")
		b.WriteString(renderBody(item, width))
		if len(reviews) > 0 {
			b.WriteString(renderReviews(reviews))
		}
		if len(comments) > 0 {
			b.WriteString(renderComments(comments))
		}
		if err != nil {
			b.WriteString(fmt.Sprintf("\n  Error: %v\n", err))
		}
		return contentRenderedMsg{lines: strings.Split(b.String(), "\n")}
	}
}

func renderJiraContentCmd(item *provider.JiraItem, comments []provider.JiraComment, width int, childCursor int, childFocused bool, err error) tea.Cmd {
	return func() tea.Msg {
		var b strings.Builder
		b.WriteString(renderJiraHeader(item))
		b.WriteString("\n\n")
		b.WriteString(renderJiraBody(item, width))
		if len(comments) > 0 {
			b.WriteString(renderJiraComments(comments))
		}
		b.WriteString(renderJiraChildren(item, childCursor, childFocused))
		if err != nil {
			b.WriteString(fmt.Sprintf("\n  Error: %v\n", err))
		}
		return contentRenderedMsg{lines: strings.Split(b.String(), "\n")}
	}
}

func (m Model) View() tea.View {
	if m.sessionPicker {
		return m.viewSessionPicker()
	}

	statusBar := "\n"
	if m.statusText != "" {
		statusBar += statusStyle.Render("  " + m.statusText)
	} else if m.embedded {
		if m.jiraKey != "" {
			escLabel := "esc: back"
			if len(m.parentStack) > 0 {
				escLabel = "esc: parent"
			}
			var hint string
			if m.jiraItem != nil && len(m.jiraItem.Children) > 0 {
				if m.childFocused {
					hint = fmt.Sprintf("  tab: focus description | enter: open child | a: spawn agent | %s", escLabel)
				} else {
					hint = fmt.Sprintf("  tab: focus child issues | a: spawn agent | r: refresh | o: open in browser | %s", escLabel)
				}
			} else {
				hint = fmt.Sprintf("  a: spawn agent | r: refresh | o: open in browser | %s", escLabel)
			}
			statusBar += statusStyle.Render(hint)
		} else {
			statusBar += statusStyle.Render("  a: spawn agent | t: attach to session | r: refresh | o: open in browser | esc: back")
		}
	} else {
		statusBar += statusStyle.Render("  r: refresh | o: open in browser | q: quit")
	}

	if len(m.renderedLines) == 0 {
		var content string
		if m.jiraItem != nil {
			content = renderJiraHeader(m.jiraItem) + "\n\n  Loading..."
		} else {
			content = renderHeader(m.item) + "\n\n  Loading..."
		}
		contentLines := strings.Split(content, "\n")
		if m.height > 0 {
			viewHeight := m.height - 2
			for len(contentLines) < viewHeight {
				contentLines = append(contentLines, "")
			}
		}
		v := tea.NewView(strings.Join(contentLines, "\n") + statusBar)
		v.AltScreen = true
		return v
	}

	lines := m.renderedLines
	if m.scroll > 0 && m.scroll < len(lines) {
		lines = lines[m.scroll:]
	}

	if m.height > 0 {
		viewHeight := m.height - 2
		if viewHeight > 0 && len(lines) > viewHeight {
			lines = lines[:viewHeight]
		}
		for len(lines) < viewHeight {
			lines = append(lines, "")
		}
	}

	v := tea.NewView(strings.Join(lines, "\n") + statusBar)
	v.AltScreen = true
	return v
}

func (m Model) viewSessionPicker() tea.View {
	var b strings.Builder
	b.WriteString("\n  Select a session to attach to:\n\n")
	for i, s := range m.sessions {
		cursor := "  "
		if i == m.sessionCursor {
			cursor = "> "
		}
		label := s.ID
		if s.Name != "" {
			label = s.Name + " (" + s.ID + ")"
		}
		b.WriteString(fmt.Sprintf("  %s%s  [%s]  %s\n", cursor, label, s.Status, s.Agent))
	}
	b.WriteString("\n")
	b.WriteString(statusStyle.Render("  enter: select | esc: cancel"))
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m Model) maxScroll() int {
	max := len(m.renderedLines) - m.height + 2
	if max < 0 {
		return 0
	}
	return max
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.sessionPicker {
		return m.handleSessionPickerKey(msg)
	}

	switch {
	case msg.Code == tea.KeyTab:
		if m.jiraItem != nil && len(m.jiraItem.Children) > 0 {
			m.childFocused = !m.childFocused
			if m.childFocused && m.childCursor >= len(m.jiraItem.Children) {
				m.childCursor = 0
			}
			return m, renderJiraContentCmd(m.jiraItem, m.jiraComments, m.width, m.childCursor, m.childFocused, m.err)
		}
		return m, nil
	case msg.Code == tea.KeyDown:
		if m.childFocused && m.jiraItem != nil {
			if m.childCursor < len(m.jiraItem.Children)-1 {
				m.childCursor++
			}
			return m, renderJiraContentCmd(m.jiraItem, m.jiraComments, m.width, m.childCursor, m.childFocused, m.err)
		}
		if m.scroll < m.maxScroll() {
			m.scroll++
		}
		return m, nil
	case msg.Code == tea.KeyUp:
		if m.childFocused {
			if m.childCursor > 0 {
				m.childCursor--
			}
			return m, renderJiraContentCmd(m.jiraItem, m.jiraComments, m.width, m.childCursor, m.childFocused, m.err)
		}
		if m.scroll > 0 {
			m.scroll--
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.childFocused && m.jiraItem != nil && m.childCursor < len(m.jiraItem.Children) {
			child := m.jiraItem.Children[m.childCursor]
			m.parentStack = append(m.parentStack, jiraDetailState{
				item:         m.jiraItem,
				comments:     m.jiraComments,
				key:          m.jiraKey,
				scroll:       m.scroll,
				childCursor:  m.childCursor,
				childFocused: m.childFocused,
			})
			m.jiraKey = child.Key
			m.jiraItem = nil
			m.jiraComments = nil
			m.scroll = 0
			m.childCursor = 0
			m.childFocused = false
			m.renderedLines = nil
			return m, fetchJiraItemDetailCmd(m.conn, child.Key)
		}
		return m, nil
	case msg.Code == 'r':
		if m.conn != nil {
			if m.jiraKey != "" {
				return m, fetchJiraItemDetailCmd(m.conn, m.jiraKey)
			}
			return m, fetchItemCmd(m.conn, m.ref)
		}
		return m, nil
	case msg.Code == 'o':
		if m.jiraItem != nil && m.jiraItem.URL != "" {
			return m, openBrowserCmd(m.jiraItem.URL)
		}
		if m.item != nil && m.item.URL != "" {
			return m, openBrowserCmd(m.item.URL)
		}
		return m, nil
	case msg.Code == 'a':
		if m.embedded {
			if m.jiraKey != "" {
				key := m.jiraKey
				return m, func() tea.Msg { return SpawnJiraSessionMsg{Key: key} }
			}
			ref := m.ref
			return m, func() tea.Msg { return SpawnSessionMsg{Ref: ref} }
		}
		return m, nil
	case msg.Code == 't':
		if m.embedded && m.conn != nil {
			return m, fetchItemSessionsCmd(m.conn, m.ref)
		}
		return m, nil
	case msg.Code == 'q' || msg.Code == tea.KeyEscape:
		if len(m.parentStack) > 0 {
			prev := m.parentStack[len(m.parentStack)-1]
			m.parentStack = m.parentStack[:len(m.parentStack)-1]
			m.jiraItem = prev.item
			m.jiraComments = prev.comments
			m.jiraKey = prev.key
			m.scroll = prev.scroll
			m.childCursor = prev.childCursor
			m.childFocused = prev.childFocused
			return m, renderJiraContentCmd(m.jiraItem, m.jiraComments, m.width, m.childCursor, m.childFocused, m.err)
		}
		if m.embedded {
			return m, func() tea.Msg { return CloseMsg{} }
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleSessionPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyDown || msg.Code == 'j':
		if m.sessionCursor < len(m.sessions)-1 {
			m.sessionCursor++
		}
		return m, nil
	case msg.Code == tea.KeyUp || msg.Code == 'k':
		if m.sessionCursor > 0 {
			m.sessionCursor--
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		sess := m.sessions[m.sessionCursor]
		m.sessionPicker = false
		return m, func() tea.Msg {
			return AttachSessionMsg{SessionID: sess.ID, Name: sess.Name, Status: sess.Status}
		}
	case msg.Code == tea.KeyEscape:
		m.sessionPicker = false
		return m, nil
	}
	return m, nil
}

func fetchItemSessionsCmd(conn protocol.Conn, ref Ref) tea.Cmd {
	return func() tea.Msg {
		resp, err := protocol.SendRequest(conn, protocol.MsgSessionList, "attach-sessions", nil, protocol.DefaultTimeout)
		if err != nil {
			return statusTextMsg{text: "Failed to fetch sessions"}
		}
		payload, err := protocol.ParsePayload[protocol.SessionListPayload](resp)
		if err != nil {
			return statusTextMsg{text: "Failed to parse sessions"}
		}
		var filtered []protocol.SessionPayload
		if ref.Key != "" {
			for _, s := range payload.Sessions {
				if s.ItemKey == ref.Key {
					filtered = append(filtered, s)
				}
			}
		} else {
			repo := ref.Owner + "/" + ref.Repo
			for _, s := range payload.Sessions {
				if s.Repo == repo && s.Number == ref.Number {
					filtered = append(filtered, s)
				}
			}
		}
		return sessionsLoadedMsg{sessions: filtered}
	}
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if err := browser.OpenCommand(url).Run(); err != nil {
			return statusTextMsg{text: "Failed to open browser"}
		}
		return statusTextMsg{text: "Opened in browser"}
	}
}

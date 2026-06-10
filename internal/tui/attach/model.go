package attach

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	browser "github.com/creydr/ai-mux/internal/action/browser"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
	"github.com/creydr/ai-mux/internal/tui"
)

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

func (m Model) Init() tea.Cmd {
	if m.conn == nil && m.item == nil {
		return nil
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

func (m Model) View() tea.View {
	if m.sessionPicker {
		return m.viewSessionPicker()
	}

	statusBar := "\n"
	if m.statusText != "" {
		statusBar += statusStyle.Render("  " + m.statusText)
	} else if m.embedded {
		statusBar += statusStyle.Render("  a: spawn agent | t: attach to session | r: refresh | o: open in browser | esc: back")
	} else {
		statusBar += statusStyle.Render("  r: refresh | o: open in browser | q: quit")
	}

	if len(m.renderedLines) == 0 {
		content := renderHeader(m.item) + "\n\n  Loading..."
		v := tea.NewView(content + statusBar)
		v.AltScreen = true
		return v
	}

	lines := m.renderedLines
	if m.scroll > 0 && m.scroll < len(lines) {
		lines = lines[m.scroll:]
	}

	viewHeight := m.height - 2
	if viewHeight > 0 && len(lines) > viewHeight {
		lines = lines[:viewHeight]
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
	case msg.Code == tea.KeyDown:
		if m.scroll < m.maxScroll() {
			m.scroll++
		}
		return m, nil
	case msg.Code == tea.KeyUp:
		if m.scroll > 0 {
			m.scroll--
		}
		return m, nil
	case msg.Code == 'r':
		if m.conn != nil {
			return m, fetchItemCmd(m.conn, m.ref)
		}
		return m, nil
	case msg.Code == 'o':
		if m.item != nil && m.item.URL != "" {
			return m, openBrowserCmd(m.item.URL)
		}
		return m, nil
	case msg.Code == 'a':
		if m.embedded {
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
		req, _ := protocol.NewRequest(protocol.MsgSessionList, "attach-sessions", nil)
		if err := conn.Send(req); err != nil {
			return statusTextMsg{text: "Failed to fetch sessions"}
		}
		resp, err := conn.Receive()
		if err != nil {
			return statusTextMsg{text: "Failed to fetch sessions"}
		}
		var payload protocol.SessionListPayload
		if err := json.Unmarshal(resp.Payload, &payload); err != nil {
			return statusTextMsg{text: "Failed to parse sessions"}
		}
		repo := ref.Owner + "/" + ref.Repo
		var filtered []protocol.SessionPayload
		for _, s := range payload.Sessions {
			if s.Repo == repo && s.Number == ref.Number {
				filtered = append(filtered, s)
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

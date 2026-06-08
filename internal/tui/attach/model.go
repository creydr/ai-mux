package attach

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	browser "github.com/creydr/ai-mux/internal/action/browser"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
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
}

func New(conn protocol.Conn, ref Ref) Model {
	return Model{
		conn: conn,
		ref:  ref,
	}
}

func NewEmbedded(conn protocol.Conn, ref Ref, width, height int) Model {
	return Model{
		conn:     conn,
		ref:      ref,
		embedded: true,
		width:    width,
		height:   height,
	}
}

func (m Model) Init() tea.Cmd {
	if m.conn == nil {
		return nil
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
		return m, nil
	case itemLoadedMsg:
		m.item = msg.item
		return m, nil
	case reviewsLoadedMsg:
		m.reviews = msg.reviews
		return m, nil
	case commentsLoadedMsg:
		m.comments = msg.comments
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	}
	return m, nil
}

func (m Model) View() tea.View {
	var b strings.Builder

	b.WriteString(renderHeader(m.item))
	b.WriteString("\n\n")
	b.WriteString(renderBody(m.item, m.width))

	if len(m.reviews) > 0 {
		b.WriteString(renderReviews(m.reviews))
	}
	if len(m.comments) > 0 {
		b.WriteString(renderComments(m.comments))
	}

	if m.err != nil {
		b.WriteString(fmt.Sprintf("\n  Error: %v\n", m.err))
	}

	b.WriteString("\n")
	if m.embedded {
		b.WriteString(statusStyle.Render("  c: agent | j/k: scroll | r: refresh | o: browser | esc: back"))
	} else {
		b.WriteString(statusStyle.Render("  j/k: scroll | r: refresh | o: open in browser | q: quit"))
	}

	content := b.String()
	lines := strings.Split(content, "\n")
	if m.scroll > 0 && m.scroll < len(lines) {
		lines = lines[m.scroll:]
	}

	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		m.scroll++
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
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
	case msg.Code == 'c':
		if m.embedded {
			ref := m.ref
			return m, func() tea.Msg { return SpawnSessionMsg{Ref: ref} }
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

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		browser.OpenCommand(url).Run()
		return nil
	}
}

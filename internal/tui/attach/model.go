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

	renderedLines []string
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
	case errMsg:
		m.err = msg.err
		return m, renderContentCmd(m.item, m.reviews, m.comments, m.width, m.err)
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
	statusBar := "\n"
	if m.embedded {
		statusBar += statusStyle.Render("  c: agent | r: refresh | o: browser | esc: back")
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

func (m Model) maxScroll() int {
	max := len(m.renderedLines) - m.height + 2
	if max < 0 {
		return 0
	}
	return max
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

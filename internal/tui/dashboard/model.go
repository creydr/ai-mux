package dashboard

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/protocol"
	"github.com/creydr/ai-mux/internal/provider"
)

type Model struct {
	conn   protocol.Conn
	width  int
	height int

	activeTab tab
	cursor    int
	issues    []provider.Item
	prs       []provider.Item

	issueBadge int
	prBadge    int
	err        error
}

func New(conn protocol.Conn) Model {
	return Model{
		conn: conn,
	}
}

func (m Model) Init() tea.Cmd {
	if m.conn == nil {
		return nil
	}
	return fetchItemsCmd(m.conn)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case itemsReceivedMsg:
		m.issues = msg.issues
		m.prs = msg.prs
		m.cursor = 0
		return m, nil
	case eventReceivedMsg:
		m.handleEvent(msg.event)
		return m, listenEventsCmd(m.conn)
	case errMsg:
		m.err = msg.err
		return m, nil
	case connectResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, fetchItemsCmd(m.conn)
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

	switch m.activeTab {
	case tabIssues:
		b.WriteString(renderItemList(m.issues, m.cursor, width))
	case tabPRs:
		b.WriteString(renderItemList(m.prs, m.cursor, width))
	}

	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render("  tab: switch tabs | j/k: navigate | o: open in browser | q: quit"))

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyTab:
		m.activeTab = (m.activeTab + 1) % tab(len(tabNames))
		m.cursor = 0
		if m.activeTab == tabIssues {
			m.issueBadge = 0
		} else {
			m.prBadge = 0
		}
		return m, nil
	case msg.Code == 'j' || msg.Code == tea.KeyDown:
		items := m.currentItems()
		if m.cursor < len(items)-1 {
			m.cursor++
		}
		return m, nil
	case msg.Code == 'k' || msg.Code == tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case msg.Code == 'o' || msg.Code == tea.KeyEnter:
		item := m.selectedItem()
		if item != nil && item.URL != "" {
			return m, openBrowserCmd(item.URL)
		}
		return m, nil
	case msg.Code == 'r':
		if m.conn != nil {
			return m, fetchItemsCmd(m.conn)
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
	if m.activeTab == tabIssues {
		return m.issues
	}
	return m.prs
}

func (m Model) selectedItem() *provider.Item {
	items := m.currentItems()
	if m.cursor >= 0 && m.cursor < len(items) {
		return &items[m.cursor]
	}
	return nil
}

func (m Model) updateItem(items []provider.Item, updated provider.Item) {
	for i, item := range items {
		if item.ID == updated.ID {
			items[i] = updated
			return
		}
	}
}

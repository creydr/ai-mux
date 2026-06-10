package dashboard

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/creydr/ai-mux/internal/event"
	"github.com/creydr/ai-mux/internal/protocol"
)

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
	case event.TypeSessionStatus:
		if ev.Session != nil {
			m.handleSessionEvent(*ev.Session)
		}
	}
}

func (m *Model) handleSessionEvent(sess protocol.SessionPayload) {
	switch sess.Status {
	case "completed", "failed", "stopped":
		for i, s := range m.sessions {
			if s.ID == sess.ID {
				m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
				if m.sessionCursor >= len(m.sessions) && m.sessionCursor > 0 {
					m.sessionCursor--
				}
				break
			}
		}
	default:
		for i, s := range m.sessions {
			if s.ID == sess.ID {
				m.sessions[i] = sess
				return
			}
		}
		m.sessions = append(m.sessions, sess)
	}
	if m.activeTab != tabSessions {
		m.sessionBadge++
	}
}

func (m *Model) startSessionTick() tea.Cmd {
	m.sessionTickID++
	id := m.sessionTickID
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return sessionTickMsg{id: id}
	})
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
	if m.renameActive {
		return "enter: confirm | esc: cancel"
	}
	switch m.activeTab {
	case tabSessions:
		return "enter: attach | n: rename | s: stop | tab: switch | ?: help"
	default:
		bar := "a: spawn agent"
		if item := m.selectedItem(); item != nil {
			repo := item.Repo.String()
			for _, sess := range m.sessions {
				if sess.Repo == repo && sess.Number == item.Number {
					bar += " | t: attach"
					break
				}
			}
		}
		bar += " | b: open in browser | tab: switch | r: refresh | ?: help"
		return bar
	}
}

# Jira Integration — Implementation Plan

Based on design document: `docs/plans/2026-06-11-jira-integration-design.md`

## Task 1: Config — Add JiraConfig

**Files:** `internal/config/config.go`, `internal/config/config_test.go`

### config.go

Add `JiraConfig` struct and field to `Config`:

```go
type JiraConfig struct {
    JQL        string `yaml:"jql"`
    OrderBy    string `yaml:"orderBy"`
    MaxResults int    `yaml:"maxResults"`
}
```

Add to `Config` struct:
```go
Jira *JiraConfig `yaml:"jira,omitempty"`
```

Using a pointer so the zero value (nil) means "not configured" — the Jira tab
won't appear unless `jira:` is present in the config.

In `Validate()`, add after the `defaultAgent` check:
```go
if c.Jira != nil {
    if c.Jira.JQL == "" {
        return fmt.Errorf("jira.jql must be set when jira is configured")
    }
    if c.Jira.MaxResults <= 0 {
        c.Jira.MaxResults = 50
    }
}
```

### config_test.go

Add tests:
- `TestLoad_JiraConfig` — loads a config with jira section, verifies fields
- `TestValidate_JiraMissingJQL` — validates error when jql is empty
- `TestValidate_JiraDefaultMaxResults` — verifies default of 50
- `TestLoad_NoJiraConfig` — confirms `Jira` is nil when omitted

### Verification
```sh
go test ./internal/config/...
```

---

## Task 2: Provider — JiraItem and JiraComment types

**Files:** `internal/provider/jira.go` (new)

Create `internal/provider/jira.go`:

```go
package provider

import "time"

const ItemTypeJira ItemType = "jira"

type JiraItem struct {
    Key         string    `json:"key"`
    Summary     string    `json:"summary"`
    Description string    `json:"description"`
    Status      string    `json:"status"`
    Priority    string    `json:"priority"`
    Type        string    `json:"type"`
    Assignee    string    `json:"assignee"`
    Reporter    string    `json:"reporter"`
    Labels      []string  `json:"labels"`
    URL         string    `json:"url"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type JiraComment struct {
    ID        string    `json:"id"`
    Author    string    `json:"author"`
    Body      string    `json:"body"`
    CreatedAt time.Time `json:"created_at"`
}
```

### Verification
```sh
go build ./internal/provider/...
```

---

## Task 3: Jira CLI Client — acli wrapper

**Files:** `internal/provider/jira/client.go` (new),
`internal/provider/jira/client_test.go` (new)

Create a `jira` sub-package under `provider` that wraps the `acli` CLI.

### client.go

```go
package jira

import (
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
    "time"

    "github.com/creydr/ai-mux/internal/provider"
)

type Client struct{}

func NewClient() *Client {
    return &Client{}
}
```

Key methods:

**Search(ctx, jql, orderBy string, limit, startAt int) ([]provider.JiraItem, error)**
- Builds JQL: if `orderBy != ""`, appends `ORDER BY <orderBy>` to jql
- Runs: `acli jira workitem search --jql "..." --limit <limit> --json`
- If startAt > 0, uses pagination offset (acli flag or post-filter)
- Parses the JSON output into `[]provider.JiraItem`
- Maps acli JSON fields to `JiraItem` fields

**GetItem(ctx, key string) (*provider.JiraItem, error)**
- Runs: `acli jira workitem view <key> --fields "*all" --json`
- Parses JSON into `*provider.JiraItem`

**GetComments(ctx, key string) ([]provider.JiraComment, error)**
- Runs: `acli jira workitem comment list --key <key> --json`
- Parses JSON into `[]provider.JiraComment`

Each method uses `exec.CommandContext` for cancellation. Errors from `acli`
(non-zero exit, stderr) are wrapped with context.

### client_test.go

Test the JSON parsing logic using `exec.Command` stubs or by testing the
mapping functions in isolation. Create helper functions:
- `TestParseSearchResult` — unit test for JSON-to-JiraItem mapping
- `TestParseViewResult` — unit test for single item parsing
- `TestParseComments` — unit test for comment parsing

### Verification
```sh
go test ./internal/provider/jira/...
```

---

## Task 4: Protocol — Jira message types and payloads

**Files:** `internal/protocol/protocol.go`, `internal/protocol/message.go`

### protocol.go

Add new message type constants:
```go
MsgListJiraItems   MessageType = "list_jira_items"
MsgGetJiraItem     MessageType = "get_jira_item"
MsgGetJiraComments MessageType = "get_jira_comments"
```

### message.go

Add new payload types:
```go
type JiraListPayload struct {
    Offset int `json:"offset,omitempty"`
    Limit  int `json:"limit,omitempty"`
}

type JiraItemsPayload struct {
    Items json.RawMessage `json:"items"`
    Total int             `json:"total"`
}

type JiraKeyPayload struct {
    Key string `json:"key"`
}

type JiraCommentsPayload struct {
    Comments json.RawMessage `json:"comments"`
}
```

Also add `ItemKey` to `SessionSpawnPayload`:
```go
type SessionSpawnPayload struct {
    Repo           string `json:"repo"`
    Number         int    `json:"number"`
    ItemType       string `json:"item_type"`
    ItemKey        string `json:"item_key,omitempty"`
    Agent          string `json:"agent"`
    WorktreeAction string `json:"worktree_action,omitempty"`
}
```

Add `ItemKey` to `SessionPayload`:
```go
type SessionPayload struct {
    // ... existing fields ...
    ItemKey  string `json:"item_key,omitempty"`
}
```

### Verification
```sh
go build ./internal/protocol/...
go test ./internal/protocol/...
```

---

## Task 5: Session — Support Jira item key

**Files:** `internal/session/session.go`, `internal/session/manager.go`,
`internal/session/manager_test.go`

### session.go

Add `ItemKey` field to `Session`:
```go
type Session struct {
    // ... existing fields ...
    ItemKey  string `json:"item_key,omitempty"`
}
```

Update `generateID` to handle Jira keys:
```go
func generateIDForKey(prefix, key string) string {
    b := make([]byte, 4)
    rand.Read(b)
    return fmt.Sprintf("%s-%s-%x", prefix, key, b)
}
```

### manager.go

Add `SpawnForJira` method (or extend `Spawn` to accept `ItemKey`). The cleanest
approach: add an `ItemKey` parameter to `Spawn`. When `ItemKey != ""`, use it
for worktree naming instead of `ItemNumber`:

```go
func (m *Manager) Spawn(itemRepo string, itemNumber int, itemType string,
    itemKey string, agentName string, wtAction WorktreeAction) (*Session, error) {
```

When `itemType == "jira"`:
- `wtName` = `jira-<sanitized-agent>-<key>` (e.g. `jira-claude-PROJ-123`)
- `id` = `generateIDForKey("jira", key)`
- `sess.ItemKey = itemKey`
- Always use `m.worktrees.Create(...)` (not `CreateForPR`)

Update `WorktreeExists` to also accept an `itemKey` parameter for Jira items.

Update `FindByItem` to also match by `ItemKey` when searching for Jira sessions.

### manager_test.go

Add tests:
- `TestManager_SpawnJira` — spawn with itemType="jira", itemKey="PROJ-123"
- `TestManager_SpawnJira_WorktreeNaming` — verify worktree name is `jira-claude-PROJ-123`
- `TestManager_FindByItemKey` — find session by Jira key

### Verification
```sh
go test ./internal/session/...
```

---

## Task 6: Daemon — Jira polling and message handlers

**Files:** `internal/daemon/daemon.go`, `internal/daemon/jira.go` (new),
`internal/daemon/daemon_test.go`

### daemon.go

Add `jiraClient` field to `Daemon`:
```go
type Daemon struct {
    // ... existing fields ...
    jiraClient *jira.Client
    jiraItems  []provider.JiraItem
    jiraMu     sync.RWMutex
}
```

In `New()`, if `cfg.Jira != nil`, create the Jira client:
```go
if cfg.Jira != nil {
    d.jiraClient = jira.NewClient()
}
```

In `Start()`, launch Jira polling alongside GitHub polling:
```go
if d.jiraClient != nil {
    go d.pollJira(ctx)
}
```

Add Jira message types to `handleMessage`:
```go
case protocol.MsgListJiraItems:
    d.handleListJiraItems(cc, msg)
case protocol.MsgGetJiraItem:
    d.handleGetJiraItem(cc, msg)
case protocol.MsgGetJiraComments:
    d.handleGetJiraComments(cc, msg)
```

Update `handleSessionSpawn` to pass `ItemKey` through to session manager.

Update `sessionToPayload` to include `ItemKey`.

### jira.go (new)

Contains the polling loop and message handlers:

**pollJira(ctx)** — Runs on `pollInterval`, calls `jiraClient.Search()`,
stores results in `d.jiraItems`, publishes events for new/updated items.

**handleListJiraItems** — Returns cached items with offset/limit pagination.

**handleGetJiraItem** — Calls `jiraClient.GetItem(key)` and returns the result.

**handleGetJiraComments** — Calls `jiraClient.GetComments(key)` and returns.

### daemon_test.go

Add tests:
- `TestDaemon_ListJiraItems` — verify list response
- `TestDaemon_GetJiraItem` — verify single item fetch
- (Mock the jira client for tests)

### Verification
```sh
go test ./internal/daemon/...
```

---

## Task 7: Event — Add Jira event types

**Files:** `internal/event/event.go`

Add new event types:
```go
TypeNewJiraItem     Type = "new_jira_item"
TypeJiraItemUpdated Type = "jira_item_updated"
```

Add `JiraItem` field to `Event`:
```go
type Event struct {
    // ... existing fields ...
    JiraItem *provider.JiraItem `json:"jira_item,omitempty"`
}
```

### Verification
```sh
go build ./internal/event/...
```

---

## Task 8: Dashboard — Dynamic tab system with Jira tab

**Files:** `internal/tui/dashboard/tabs.go`, `internal/tui/dashboard/model.go`

### tabs.go

Replace the hardcoded tab enum with a dynamic system:

```go
type tab int

const (
    tabIssues tab = iota
    tabPRs
    tabJira
    tabSessions
)
```

Update `tabNames` and `renderTabs` to accept a `jiraEnabled bool` parameter:

```go
func tabConfig(jiraEnabled bool) ([]string, []tab) {
    names := []string{"Issues", "Pull Requests"}
    tabs := []tab{tabIssues, tabPRs}
    if jiraEnabled {
        names = append(names, "Jira")
        tabs = append(tabs, tabJira)
    }
    names = append(names, "Sessions")
    tabs = append(tabs, tabSessions)
    return names, tabs
}
```

Update `renderTabs` to use the dynamic config and accept jiraBadge.

### model.go

Add to `Model`:
```go
jiraEnabled   bool
jiraItems     []provider.JiraItem
jiraCursor    int
jiraBadge     int
jiraHasMore   bool
jiraOffset    int
repoNames     []string   // repo names from config, for repo picker
repoPicker    bool       // true when repo picker is showing
repoPickerCursor int
```

Update `New()` to accept `jiraEnabled bool` and `repoNames []string`.

Update tab switching logic to use dynamic tab list.

Update `rebuildViewport` to handle `tabJira` — full width, flat list.

### Verification
```sh
go test ./internal/tui/dashboard/...
```

---

## Task 9: Dashboard — Jira list view rendering

**Files:** `internal/tui/dashboard/itemlist.go`,
`internal/tui/dashboard/data.go`

### itemlist.go

Add `buildJiraContentLines` function:
```go
func buildJiraContentLines(items []provider.JiraItem, cursor, width int,
    hasMore bool, sessions []protocol.SessionPayload) ([]string, int) {
```

Each row:
```
  PROJ-123  Summary text here                    [In Progress] [High]
```

Format: `  %-10s  %-*s  %s %s` — key left-aligned, summary fills space, status
and priority as right-aligned styled badges.

Session badges shown by matching `sess.ItemKey == item.Key`.

At the end, if `hasMore`, append a `[more...]` row that the user can select.

Add Jira-specific styles to `styles.go`:
```go
jiraStatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF"))
jiraPriorityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB86C"))
```

### data.go

Add `selectedJiraItem()`:
```go
func (m Model) selectedJiraItem() *provider.JiraItem {
    if m.jiraCursor >= 0 && m.jiraCursor < len(m.jiraItems) {
        return &m.jiraItems[m.jiraCursor]
    }
    return nil
}
```

### Verification
```sh
go test ./internal/tui/dashboard/...
```

---

## Task 10: Dashboard — Jira commands and messages

**Files:** `internal/tui/dashboard/commands.go`,
`internal/tui/dashboard/messages.go`

### messages.go

Add new message types:
```go
type jiraItemsReceivedMsg struct {
    items   []provider.JiraItem
    total   int
    offset  int
}

type jiraItemDetailMsg struct {
    item     *provider.JiraItem
    comments []provider.JiraComment
}
```

### commands.go

Add commands:

**fetchJiraItemsCmd(conn, offset, limit)** — sends `MsgListJiraItems`, parses
response into `jiraItemsReceivedMsg`.

**fetchJiraItemDetailCmd(conn, key)** — sends `MsgGetJiraItem` and
`MsgGetJiraComments`, returns `jiraItemDetailMsg`.

### Verification
```sh
go build ./internal/tui/dashboard/...
```

---

## Task 11: Dashboard — Key handling for Jira tab

**Files:** `internal/tui/dashboard/keys.go`

Update `handleKey` for the Jira tab:

**Tab switching:** Update to use the dynamic tab list. Clear `jiraCursor` on
switch. Set `jiraBadge = 0` when switching to Jira tab.

**Navigation (j/k):** When `activeTab == tabJira`, navigate `jiraCursor`.

**Enter:** When on Jira tab:
- If cursor is on `[more...]`, fetch next page (offset += maxResults)
- Otherwise, open Jira detail view

**`a` key:** When on Jira tab:
- No agents → show error
- Get selected Jira item
- If >1 repo configured → show repo picker → then agent picker
- If 1 repo → skip repo picker, show agent picker or spawn directly

**`o` key:** Open `jiraItem.URL` in browser.

**`t` key:** Attach to session matching this Jira key.

**`s` key:** Stop session matching this Jira key.

### Verification
```sh
go test ./internal/tui/dashboard/...
```

---

## Task 12: Dashboard — Repo picker modal

**Files:** `internal/tui/dashboard/pickers.go`, `internal/tui/dashboard/model.go`

Add `viewRepoPicker` to `viewState` enum.

### pickers.go

Add repo picker handler and renderer, following the same pattern as the agent
picker:

```go
func (m Model) handleRepoPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
    // j/k navigation, Enter to select, Esc to cancel
}

func (m Model) renderRepoPicker() tea.View {
    // Title: "Select Repository"
    // List repo names from config
    // Same visual style as agent picker
}
```

After repo selection:
- Set `pendingSpawn.repo` to selected repo
- If `defaultAgent` set → spawn directly
- Else → show agent picker

### model.go

Add `repoPickerCursor int` and update `viewState` enum with `viewRepoPicker`.

Update `spawnRequest` to handle Jira:
```go
type spawnRequest struct {
    repo     string
    number   int
    itemType string
    itemKey  string
    agent    string
}
```

### Verification
```sh
go test ./internal/tui/dashboard/...
```

---

## Task 13: Dashboard — Event handling for Jira

**Files:** `internal/tui/dashboard/events.go`

Add Jira event handling to `handleEvent`:
```go
case event.TypeNewJiraItem:
    if ev.JiraItem != nil {
        m.jiraItems = append(m.jiraItems, *ev.JiraItem)
        if m.activeTab != tabJira {
            m.jiraBadge++
        }
    }
case event.TypeJiraItemUpdated:
    if ev.JiraItem != nil {
        m.updateJiraItem(*ev.JiraItem)
    }
```

Add `updateJiraItem` helper to `data.go`:
```go
func (m *Model) updateJiraItem(updated provider.JiraItem) {
    for i, item := range m.jiraItems {
        if item.Key == updated.Key {
            m.jiraItems[i] = updated
            return
        }
    }
}
```

### Verification
```sh
go test ./internal/tui/dashboard/...
```

---

## Task 14: Dashboard — Status bar and help for Jira

**Files:** `internal/tui/dashboard/events.go` (statusBarText),
`internal/tui/dashboard/help.go`

### events.go

Update `statusBarText()` to add Jira tab case:
```go
case tabJira:
    bar := "a: spawn agent"
    if item := m.selectedJiraItem(); item != nil {
        for _, sess := range m.sessions {
            if sess.ItemKey == item.Key {
                bar += " | t: attach"
                break
            }
        }
    }
    bar += " | b: open in browser | tab: switch | r: refresh | ?: help"
    return bar
```

### help.go

Add a "Jira" section to the help overlay:
```go
section("Jira", [][2]string{
    {"a", "Spawn agent (select repo first)"},
    {"t", "Attach to session"},
    {"b / o", "Open in browser"},
    {"s", "Stop session"},
    {"Enter", "View details / load more"},
    {"r", "Refresh"},
})
```

### Verification
```sh
go test ./internal/tui/dashboard/...
```

---

## Task 15: Detail view — Jira item support

**Files:** `internal/tui/attach/model.go`, `internal/tui/attach/render.go`,
`internal/tui/attach/parse.go`, `internal/tui/attach/messages.go`,
`internal/tui/attach/commands.go`

### parse.go

Add `JiraRef` type:
```go
type JiraRef struct {
    Key string
}
```

### messages.go

Add Jira-specific messages:
```go
type jiraItemLoadedMsg struct {
    item     *provider.JiraItem
    comments []provider.JiraComment
}

type SpawnJiraSessionMsg struct {
    Key string
}
```

### model.go

Extend the Model to support Jira items alongside GitHub items. Add fields:
```go
jiraRef      *JiraRef
jiraItem     *provider.JiraItem
jiraComments []provider.JiraComment
```

Add `NewEmbeddedJira` constructor:
```go
func NewEmbeddedJira(conn protocol.Conn, key string, width, height int,
    item *provider.JiraItem) Model {
```

Update `Init()` to handle Jira items.
Update `Update()` to handle `jiraItemLoadedMsg`.
Update `View()` to render Jira content when `jiraRef != nil`.
Update `handleKey()` — `a` emits `SpawnJiraSessionMsg`, `o` opens `jiraItem.URL`.

### render.go

Add Jira rendering functions:
```go
func renderJiraHeader(item *provider.JiraItem) string {
    // Key: Summary
    // Status: X    Priority: Y    Type: Z
    // Assignee: A  Reporter: B
    // Labels: ...
}

func renderJiraBody(item *provider.JiraItem, width int) string {
    // Description rendered as markdown via glamour
}

func renderJiraComments(comments []provider.JiraComment) string {
    // Same pattern as renderComments but with JiraComment
}
```

### commands.go

Add `fetchJiraItemCmd`:
```go
func fetchJiraItemCmd(conn protocol.Conn, key string) tea.Cmd {
    // Send MsgGetJiraItem, then MsgGetJiraComments
    // Return jiraItemLoadedMsg
}
```

### Verification
```sh
go test ./internal/tui/attach/...
```

---

## Task 16: Dashboard — Wire up Jira detail view from dashboard

**Files:** `internal/tui/dashboard/keys.go`, `internal/tui/dashboard/model.go`

In `handleKey`, when Enter is pressed on a Jira item:
```go
if m.activeTab == tabJira {
    item := m.selectedJiraItem()
    if item != nil {
        detail := attach.NewEmbeddedJira(m.conn, item.Key, m.width, m.height, item)
        m.itemDetail = &detail
        m.view = viewItemDetail
        return m, m.itemDetail.Init()
    }
}
```

Handle `SpawnJiraSessionMsg` in `Update()`:
```go
case attach.SpawnJiraSessionMsg:
    // Same as SpawnSessionMsg but with repo picker flow
    // and itemKey instead of number
```

### Verification
```sh
go test ./internal/tui/dashboard/...
```

---

## Task 17: Dashboard — Init and View integration

**Files:** `internal/tui/dashboard/model.go`, `internal/tui/dashboard/commands.go`

Update `Init()` to also fetch Jira items if enabled:
```go
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
```

Update `View()` to handle Jira tab rendering — full width (no side panel),
like Sessions tab:
```go
} else if m.activeTab == tabJira {
    if len(m.jiraItems) == 0 {
        b.WriteString(statusBarStyle.Render("  No Jira items"))
    } else {
        b.WriteString(m.viewport.View())
    }
}
```

Update `rebuildViewport()` for Jira:
```go
if m.activeTab == tabJira {
    m.viewport.SetWidth(width)
    m.viewport.SetHeight(vpHeight)
    lines, cursorLine := buildJiraContentLines(m.jiraItems, m.jiraCursor, width, m.jiraHasMore, m.sessions)
    m.viewport.SetContentLines(lines)
    m.cursorLine = cursorLine
}
```

Update `Update()` to handle `jiraItemsReceivedMsg`:
```go
case jiraItemsReceivedMsg:
    if msg.offset == 0 {
        m.jiraItems = msg.items
    } else {
        m.jiraItems = append(m.jiraItems, msg.items...)
    }
    m.jiraHasMore = len(m.jiraItems) < msg.total
    m.jiraOffset = msg.offset + len(msg.items)
    m.rebuildViewport()
    return m, nil
```

### Verification
```sh
go test ./internal/tui/dashboard/...
```

---

## Task 18: Dashboard cmd — Pass Jira config through

**Files:** `cmd/ai-mux/commands/dashboard.go` (or wherever the dashboard
command wires config to the TUI model)

Pass `cfg.Jira != nil` as `jiraEnabled` and repo names list to `dashboard.New()`.

### Verification
```sh
go build ./cmd/ai-mux/...
```

---

## Task 19: Documentation updates

**Files:** `README.md`

Update:
- Requirements section: add `acli` (Atlassian CLI) as optional dependency
- Quick Start config example: add `jira:` section as optional
- Keyboard shortcuts: add Jira tab section
- Configuration Reference table: add `jira.jql`, `jira.orderBy`, `jira.maxResults`
- Worktree section: mention Jira worktree naming pattern

### Verification
Read through the rendered README.

---

## Task Dependency Order

```
Task 1 (config) ─┐
Task 2 (types)  ─┤
Task 7 (events) ─┼─► Task 3 (acli client) ─► Task 6 (daemon)
Task 4 (proto)  ─┤
Task 5 (session) ┘
                                               Task 6 ─┐
Task 8 (tabs)   ────────────────────────────────────────┤
Task 9 (list view) ────────────────────────────────────┤
Task 10 (commands/messages) ───────────────────────────┤
Task 11 (keys) ────────────────────────────────────────┼─► Task 17 (integration)
Task 12 (repo picker) ────────────────────────────────┤     ─► Task 18 (cmd)
Task 13 (events) ──────────────────────────────────────┤     ─► Task 19 (docs)
Task 14 (status/help) ────────────────────────────────┤
Task 15 (detail view) ────────────────────────────────┤
Task 16 (detail wiring) ──────────────────────────────┘
```

Tasks 1-5, 7-8 can be done in parallel as they touch independent files.
Tasks 6, 9-16 depend on the foundation layers.
Tasks 17-19 are final integration and polish.

## Commit Strategy

Each task should be its own commit to maintain a clean history:
1. `Add JiraConfig to config with validation`
2. `Add JiraItem and JiraComment provider types`
3. `Add Jira acli client wrapper`
4. `Add Jira protocol message types and payloads`
5. `Add ItemKey support to session manager`
6. `Add Jira event types`
7. `Add Jira polling and message handlers to daemon`
8. `Add dynamic tab system with Jira tab`
9. `Add Jira list view rendering`
10. `Add Jira dashboard commands and messages`
11. `Add Jira key handling in dashboard`
12. `Add repo picker modal for Jira sessions`
13. `Add Jira event handling in dashboard`
14. `Add Jira status bar and help section`
15. `Add Jira detail view rendering`
16. `Wire Jira detail view to dashboard`
17. `Integrate Jira tab in dashboard init and view`
18. `Pass Jira config through dashboard command`
19. `Update documentation for Jira integration`

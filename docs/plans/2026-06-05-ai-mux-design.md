# ai-mux Design Document

## Overview

**ai-mux** is a Go CLI tool for monitoring multiple GitHub repositories. It watches for new issues, PRs, and review activity, and provides actionable options — including spawning AI agent sessions (Claude, Gemini, etc.) to fix issues or review PRs. It integrates with IDEs via ACP (Agent Client Protocol) for viewing diffs and running agent sessions within the editor.

## Architecture

### Components

- **Daemon** (`ai-mux daemon start/stop`) — Long-running background process that polls GitHub using `google/go-github`. Maintains state, exposes a local Unix socket for clients.
- **Dashboard** (`ai-mux dashboard`) — Full-screen bubbletea TUI. Tabbed layout: Issues | PRs | Activity. Connects to daemon via Unix socket.
- **Attach** (`ai-mux attach <type>/<owner>/<repo>/<id>`) — Focused TUI for a single item. Connects to same daemon. Real-time updates as new comments/reviews arrive.
- **ACP Agent** — ACP-compatible subprocess for IDE integration (IntelliJ/VSCode). Pushes diffs to editor, manages agent sessions, communicates over JSON-RPC/stdio.

### Communication

```
┌─────────┐     Unix Socket     ┌────────┐     go-github     ┌────────┐
│Dashboard│◄───────────────────►│ Daemon │◄──────────────────►│ GitHub │
│ Attach  │                     │        │                    │  API   │
└─────────┘                     └────┬───┘                    └────────┘
                                     │
┌─────────┐     JSON-RPC/stdio  ┌────┴───┐
│  IDE    │◄───────────────────►│  ACP   │
│         │                     │ Agent  │
└─────────┘                     └────────┘
```

## Internal Design & Extensibility

Each layer communicates through Go interfaces. Adding a new source (GitLab), action (deploy), or notification channel (Slack) means implementing an interface, not modifying existing code.

### Package Structure

```
ai-mux/
├── cmd/
│   └── ai-mux/              # CLI entrypoint
├── internal/
│   ├── daemon/               # Daemon lifecycle, socket server
│   ├── poller/               # Polling orchestration
│   ├── provider/             # Source provider interface + implementations
│   │   ├── provider.go       # Interface: ListIssues, ListPRs, GetDiff, etc.
│   │   └── github/           # google/go-github implementation
│   ├── store/                # State persistence (seen items, read status)
│   │   ├── store.go          # Store interface
│   │   └── jsonfile/         # JSON file implementation
│   ├── notifier/             # Notification interface + implementations
│   │   ├── notifier.go       # Interface: Notify(event)
│   │   ├── desktop/          # notify-send implementation
│   │   └── tui/              # In-TUI notification
│   ├── worktree/             # Git worktree management
│   │   └── worktree.go       # Create, list, clean worktrees
│   ├── action/               # Action interface + implementations
│   │   ├── action.go         # Interface: Execute(context, item)
│   │   ├── agent/            # AI agent runner (Claude, Gemini, etc.)
│   │   ├── browser/          # Open in browser
│   │   └── assign/           # Assign to self
│   ├── protocol/             # Daemon ↔ client communication
│   │   ├── protocol.go       # Transport + Conn interfaces
│   │   └── jsonlines/        # Newline-delimited JSON over Unix socket
│   ├── event/                # Event bus and event types
│   ├── acp/                  # ACP agent implementation (JSON-RPC/stdio)
│   └── config/               # Config loading and validation
├── docs/
│   └── plans/
├── Makefile
├── README.md
└── go.mod
```

### Key Interfaces

- **`provider.Provider`** — Abstracts where items come from. GitHub today, GitLab or Gitea tomorrow.
- **`notifier.Notifier`** — Abstracts how you get alerted. Desktop, TUI badge, or future webhook.
- **`action.Action`** — Abstracts what you can do with an item. Each action is self-contained and registered by type.
- **`store.Store`** — Abstracts state persistence. Start with JSON file, swap to SQLite later if needed.
- **`protocol.Transport`** — Abstracts daemon ↔ client communication. Start with JSON lines over Unix socket, swap to gRPC later without touching daemon or client code.

### Event Bus

The daemon emits events (`NewIssue`, `NewPR`, `ReviewReceived`, etc.) through a channel-based event bus. Pollers produce events, notifiers and connected TUI clients consume them. Adding a new event type means defining it and wiring up producers/consumers.

## Dashboard TUI

### Layout

```
┌─ ai-mux ──────────────────────────────────────────────────┐
│ [Issues (3)] [PRs (5)] [Activity]            ⚡ polling 30s│
├────────────────────────────────────────────────────────────┤
│ owner/repo-a                                               │
│   ● #42  Fix login timeout          bug     2m ago    NEW │
│   ● #38  Add retry logic            feat    1h ago        │
│ owner/repo-b                                               │
│   ● #15  Broken CI on main          ci      5m ago    NEW │
├────────────────────────────────────────────────────────────┤
│ [enter] details  [c] agent fix  [b] browser  [a] assign   │
│ [t] attach in terminal  [d] view diff in IDE               │
└────────────────────────────────────────────────────────────┘
```

### Key Interactions

- **Tab** — Switch between Issues / PRs / Activity
- **j/k / arrows** — Navigate items
- **Enter** — Expand inline detail (body, comments, review status)
- **c** — Spawn AI agent session (uses default agent, or picker if multiple configured)
- **b** — Open in browser
- **t** — Print/copy `ai-mux attach ...` command for another terminal
- **d** — Send diff to IDE via ACP
- **/** — Filter by repo, label, or author

## Attach Mode

Focused view for a single item with full context:

```
┌─ ai-mux attach pr/owner/repo-a/123 ─────────────────────┐
│ PR #123: Refactor auth middleware                         │
│ by @colleague · 3 files changed · review requested       │
├───────────────────────────────────────────────────────────┤
│ Description:                                              │
│   Moves session handling to new middleware...              │
│                                                           │
│ Reviews:                                                  │
│   ✓ @teammate - approved                                  │
│   ● @you - pending                                        │
│                                                           │
│ Files: auth.go (+45 -12), middleware.go (+80 -0), ...     │
├───────────────────────────────────────────────────────────┤
│ [c] agent review  [d] diff in IDE  [b] browser            │
│ [r] reply/comment  [a] approve                            │
└───────────────────────────────────────────────────────────┘
```

Stays connected to daemon — new comments/reviews appear in real time.

## ACP Agent

Runs as IDE subprocess, communicating over JSON-RPC/stdio.

### Capabilities

- **Push diffs** — Receive file changes from daemon, present as editor diffs
- **Agent sessions** — Start, manage, display AI agent sessions within IDE's agent UI
- **Actions** — Same action set as TUI, rendered through IDE's native UI via ACP content types
- **Session continuity** — Pick up context from dashboard; dashboard reflects active IDE sessions

### Registration

Agent registers in ACP registry for IDE discovery. Configuration comes from `~/.config/ai-mux/config.yaml`.

## AI Agent Runners

Agent runners are provider-agnostic. Adding a new AI tool is a config entry, no code changes.

### Interface

```go
type AgentRunner interface {
    Name() string
    Supports(actionType ActionType) bool
    Command(ctx ActionContext) *exec.Cmd
    PostSession() PostSessionBehavior // "keep" or "auto-pr"
}
```

### Worktree Isolation

Every agent action runs in a dedicated git worktree, regardless of action type. This ensures:
- Full repo context for the agent (code navigation, tests, etc.)
- No interference with your current working branch or uncommitted changes
- Multiple actions can run in parallel on the same repo

Worktrees are created at `<repo-path>/.worktrees/<action>-<number>` (e.g., `.worktrees/fix-issue-42`, `.worktrees/review-pr-123`).

### Post-Session Behavior

After an agent session ends, the worktree cleanup depends on whether the agent produced code changes:

- **No changes** — worktree is always cleaned up automatically (typical for reviews)
- **Changes produced** — behavior is configurable per agent:
  - **`keep`** — Worktree stays on disk for manual review. `ai-mux worktree list` shows all active worktrees, `ai-mux worktree clean` removes stale ones.
  - **`auto-pr`** — Commits any uncommitted changes, pushes the branch, opens a draft PR linking to the original issue, then removes the worktree.

### Configuration

```yaml
agents:
  - name: claude
    command: claude
    post_session: auto-pr  # keep | auto-pr
    args_templates:
      fix_issue: "--print 'Fix issue #{{.Number}}: {{.Title}}'"
      review_pr: "--print 'Review PR #{{.Number}}'"

  - name: gemini
    command: gemini
    post_session: keep
    args_templates:
      fix_issue: "-p 'Fix issue #{{.Number}}: {{.Title}}'"
      review_pr: "-p 'Review PR #{{.Number}}'"

default_agent: claude
```

## Configuration

Location: `~/.config/ai-mux/config.yaml`

```yaml
repos:
  - name: owner/repo-a
    path: ~/development/repo-a
  - name: owner/repo-b
    path: ~/development/repo-b
  - name: org/repo-c
    path: ~/projects/repo-c

poll_interval: 30s

github:
  token_from: gh

notifications:
  desktop:
    enabled: false
    events:
      - review_requested
      - review_received

agents:
  - name: claude
    command: claude
    post_session: auto-pr
    args_templates:
      fix_issue: "--print 'Fix issue #{{.Number}}: {{.Title}}'"
      review_pr: "--print 'Review PR #{{.Number}}'"

default_agent: claude

acp:
  socket: /tmp/ai-mux.sock
```

## State

Location: `~/.local/state/ai-mux/state.json`

Tracks:
- Last seen issue/PR ID per repo
- Read/unread status per item
- Active agent sessions
- Active worktrees (path, repo, action, status)
- Attached clients

## Daemon ↔ Client Protocol

Newline-delimited JSON over Unix socket. Swappable to gRPC via the `Transport` interface.

### Message Format

```go
type Message struct {
    Type    string          `json:"type"`
    ID      string          `json:"id,omitempty"`
    Payload json.RawMessage `json:"payload"`
}
```

### Client → Daemon

- `subscribe` — Start receiving events (optionally filtered)
- `list_issues` / `list_prs` — Get current state
- `get_item` — Full detail for a single item
- `mark_read` — Mark as seen
- `execute_action` — Trigger an action
- `get_status` — Daemon health

### Daemon → Client

- `event` — New issue, PR, review, etc.
- `state_update` — Item changed
- `action_result` — Result of triggered action

### Extensibility

New message types are new `Type` strings with their own payload structs. Daemon ignores unrecognized types. The `protocol.Transport` interface allows swapping the underlying transport (JSON lines → gRPC) without changing any daemon or client logic.

## Notifications

Configurable per event type:
- **TUI-only** (default) — New items highlighted, counter badges
- **Desktop** (opt-in) — `notify-send` for specific events (review requested, review received)

## Implementation Priorities

1. Project scaffolding (go.mod, Makefile, README, config loading)
2. Core interfaces (provider, store, event, protocol)
3. GitHub provider (polling with go-github)
4. Daemon (event bus, socket server, state management)
5. Dashboard TUI (bubbletea, basic navigation, actions)
6. Attach mode
7. Agent runners (Claude, configurable)
8. Notifications (desktop)
9. ACP agent (IDE integration)
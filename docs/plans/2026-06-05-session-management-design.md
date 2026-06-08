# Session Management Design

## Overview

Transform ai-mux from a read-only GitHub monitoring dashboard into a session management hub. Users can spawn AI agent sessions to fix issues or review PRs, track running sessions, attach to them with full I/O, and control everything from both the TUI dashboard and IDE (via ACP).

## Architecture

### Session Execution: tmux-native

Each agent session runs inside a named tmux session (`ai-mux-<id>`). The daemon captures output via `tmux pipe-pane`, streams it to connected clients, and forwards user input via `tmux send-keys`. Sessions survive daemon restarts because tmux sessions are independent processes.

```
Dashboard ──┐                              ┌── tmux: ai-mux-fix-42
             ├── Unix Socket ── Daemon ────┤── tmux: ai-mux-rev-15
IDE (ACP) ──┘          │                   └── tmux: ai-mux-fix-8
                       │
                  SessionManager
                  (spawn/track/attach/stop)
```

### Session Model

```go
type Session struct {
    ID           string        // short unique ID (e.g., "fix-42-a3b2")
    ItemRepo     string        // "owner/repo"
    ItemNumber   int
    ItemType     string        // "issue" or "pr"
    Agent        string        // agent config name
    TmuxSession  string        // "ai-mux-<id>"
    Worktree     string        // filesystem path to git worktree
    Status       Status        // pending → running → completed | failed | stopped
    WaitingInput bool          // true when agent process is blocked on stdin
    CreatedAt    time.Time
    CompletedAt  *time.Time
    ExitCode     *int
    Error        string
}
```

### Session Lifecycle

1. **Spawn** (`c` on an item): Daemon creates git worktree, builds agent command from config template, starts tmux session, begins output capture. Status: `running`.

2. **Monitor**: Dashboard shows inline badge on the item (`[running]`, `[waiting]`, `[done]`) and lists the session in the Sessions tab with status, duration, and agent name.

3. **Attach** (Enter on a session): Full-screen takeover showing agent's terminal output. Keyboard input forwarded to tmux pane. Escape returns to overview.

4. **Complete**: Agent process exits. Daemon runs post-session handler per agent config (`keep` worktree or `auto-pr`). Status: `completed` or `failed`.

5. **Stop** (`s` on a session): Daemon sends SIGTERM to tmux pane, cleans up. Status: `stopped`.

### Waiting-for-Input Detection

The daemon periodically checks the foreground process state in the tmux pane. It reads `/proc/<pid>/stat` to determine if the process is blocked waiting on stdin. When detected, the session's `WaitingInput` flag is set and a status event is broadcast to connected clients.

### Dashboard Views

**Issues/PRs tabs** (existing, enhanced):
- Items with active sessions show inline badges: `[running]` green, `[waiting]` yellow, `[done]` dim
- New keybindings: `c` spawn agent, `b` browser, `a` assign, `s` stop session

**Sessions tab** (new):
- Lists all sessions with: ID, item reference, agent, status, duration
- Enter: attach to session (full-screen takeover)
- `s`: stop selected session

**Attach view** (new):
- Full-screen takeover when viewing a session
- Top bar: session info (ID, item ref, agent, status)
- Body: scrollable viewport with agent's terminal output
- Keyboard input forwarded to session
- Escape: detach and return to overview

### IDE Integration (ACP)

The ACP server becomes a full frontend, equal to the dashboard. It connects to the daemon over the same Unix socket and uses the same protocol messages.

Key methods:
- `session/new` — spawn a session
- `session/list` — list sessions
- `session/attach` — subscribe to output stream
- `session/prompt` — send input to a session
- `session/stop` — stop a session
- `diff/get` — get structured diff from session's worktree

### Protocol Messages

| Message | Direction | Purpose |
|---------|-----------|---------|
| `session_spawn` | client → daemon | Create session |
| `session_list` | client → daemon | List all sessions |
| `session_attach` | client → daemon | Subscribe to output stream |
| `session_detach` | client → daemon | Unsubscribe from output |
| `session_input` | client → daemon | Send input to session |
| `session_stop` | client → daemon | Stop a session |
| `session_output` | daemon → client | Streamed output chunk |
| `session_status` | daemon → client | Status change event |

### Configuration

```yaml
session:
  max_parallel: 5
  output_dir: ~/.ai-mux/sessions

agents:
  - name: claude
    command: claude
    post_session: auto-pr
    args_templates:
      fix_issue: "--print 'Fix issue #{{.Number}}: {{.Title}}'"
      review_pr: "--print 'Review PR #{{.Number}}'"

default_agent: claude
```

### Concurrency

Multiple sessions can run in parallel, limited by `session.max_parallel` (default 5). Each session is a separate tmux session with its own worktree, so there are no conflicts.

### Persistence

Sessions are persisted to `~/.ai-mux/sessions.json`. On daemon startup, the manager reconciles: discovers existing `ai-mux-*` tmux sessions and reattaches monitoring for any that are still running.

## Implementation Phases

1. **Session model + manager** — `internal/session/` package with Session type, Manager, tmux command interface, persistence
2. **Protocol + daemon** — New message types, daemon handlers, session manager initialization
3. **Dashboard sessions tab + badges** — Third tab, inline badges, action keybindings, status events
4. **Dashboard attach view** — Full-screen takeover, output streaming, input forwarding
5. **ACP real implementation** — Replace stubs with daemon proxying, add `diff/get`
6. **Simple actions** — Wire browser and assign keybindings

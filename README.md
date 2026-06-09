# ai-mux

A terminal-based tool for monitoring multiple GitHub repositories. Watches for new issues, PRs, and review activity with actionable integrations — spawn AI agent sessions to fix issues or review PRs, and interact with sessions from your IDE via ACP.

## Architecture

**ai-mux** uses a daemon/client architecture:

- **Daemon** — background process that polls GitHub, maintains state, manages sessions, and serves clients over a Unix socket
- **Dashboard** — full-screen TUI showing all watched repos with tabbed Issues/PRs/Sessions views
- **Attach** — focused TUI for a single issue or PR with markdown rendering
- **ACP Agent** — IDE integration via JSON-RPC over stdio for listing items and managing agent sessions

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Dashboard   │     │   Attach     │     │  ACP Agent   │
│   (TUI)      │     │   (TUI)      │     │  (IDE/stdio) │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       └────────────┬───────┴────────────────────┘
                    │ Unix socket (JSON lines)
              ┌─────┴──────┐
              │   Daemon   │
              │            │
              │  ┌────────┐│     ┌──────────┐
              │  │ Poller ├┼────►│  GitHub   │
              │  └────────┘│     └──────────┘
              │  ┌────────┐│     ┌──────────┐
              │  │Sessions├┼────►│  tmux    │
              │  └────────┘│     └──────────┘
              │  ┌────────┐│
              │  │ Store  ││
              │  └────────┘│
              └────────────┘
```

## Installation

```sh
git clone https://github.com/creydr/ai-mux.git
cd ai-mux
make build
```

Requires Go 1.26+ and `gh` CLI for GitHub authentication.

## Quick Start

1. Create a config file at `~/.config/ai-mux/config.yaml`:

```yaml
repos:
  - name: owner/repo-a
    path: ~/development/repo-a
  - name: owner/repo-b
    path: ~/development/repo-b

poll_interval: 30s

github:
  token_from: gh

agents:
  - name: claude
    command: claude
    post_session: keep
    args_templates:
      fix_issue: "Fix {{.Item.URL}}"
      review_pr: "Review {{.Item.URL}}"
  - name: gemini
    command: gemini
    post_session: keep
    args_templates:
      fix_issue: "-i Fix {{.Item.URL}}"
      review_pr: "-i Review {{.Item.URL}}"

default_agent: claude
```

2. Start the daemon:

```sh
ai-mux daemon start
```

3. Open the dashboard:

```sh
ai-mux dashboard
```

## Usage

### Daemon

```sh
# Start the daemon (foreground)
ai-mux daemon start --foreground

# Start the daemon (runs in foreground, use & or tmux to background)
ai-mux daemon start

# Check daemon status
ai-mux daemon status

# Stop the daemon
ai-mux daemon stop
```

Sessions are persisted to `~/.ai-mux/sessions.json` and survive daemon restarts. On startup, the daemon reconciles persisted sessions with live tmux state.

### Dashboard

```sh
ai-mux dashboard
```

Keyboard shortcuts:

**Item list (Issues/PRs tabs):**
- `Tab` — switch between Issues, PRs, and Sessions tabs
- Arrow keys — navigate items
- `Enter` — open item detail view
- `a` — spawn agent session for selected item
- `b` / `o` — open item in browser
- `r` — refresh
- `Ctrl-c` — quit

**Item detail view:**
- Arrow keys — scroll content
- `a` — spawn agent session for this item
- `o` — open in browser
- `r` — refresh
- `Esc` — back to list

**Sessions tab:**
- Arrow keys — navigate sessions
- `Enter` — attach to session (opens tmux)
- `s` — stop selected session
- `Ctrl-c` — quit

### Attach

```sh
# Attach to a specific issue
ai-mux attach issue/owner/repo/42

# Attach to a specific PR
ai-mux attach pr/owner/repo/123
```

Keyboard shortcuts:
- Arrow keys — scroll
- `a` — spawn agent session
- `o` — open in browser
- `r` — refresh
- `q` / `Esc` — quit

### ACP (IDE Integration)

```sh
# Start the ACP agent (communicates over stdin/stdout)
ai-mux acp
```

The ACP agent uses JSON-RPC 2.0 over stdio. Configure your IDE to launch `ai-mux acp` as an external tool, or interact with it directly:

```sh
echo '{"jsonrpc":"2.0","id":1,"method":"session/list","params":{}}' | ai-mux acp
```

**Supported methods:**

| Method | Description |
|--------|-------------|
| `initialize` | Handshake, returns server capabilities |
| `items/list` | List issues from watched repos |
| `session/new` | Spawn an agent session (`repo`, `number`, `itemType`, `agent`) |
| `session/list` | List active sessions |
| `session/stop` | Stop a session (`sessionId`) |
| `session/prompt` | Send input to a session (`sessionId`, `prompt`) |
| `session/attach` | Attach to session output, streams via `session/output` notifications |

**Example: spawn a session**
```json
{"jsonrpc":"2.0","id":1,"method":"session/new","params":{"repo":"owner/repo","number":42,"itemType":"issue","agent":"claude"}}
```

**Example: list sessions**
```json
{"jsonrpc":"2.0","id":2,"method":"session/list","params":{}}
```

## Configuration Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `repos` | list | required | Repositories to watch |
| `repos[].name` | string | required | Repository in `owner/repo` format |
| `repos[].path` | string | required | Local clone path (supports `~`) |
| `poll_interval` | duration | `30s` | How often to poll GitHub |
| `github.token_from` | string | `gh` | Token source: `gh` (GitHub CLI) |
| `github.token` | string | — | Direct token (not recommended) |
| `github.token_env` | string | — | Environment variable with token |
| `agents` | list | — | AI agent configurations |
| `agents[].name` | string | required | Agent identifier |
| `agents[].command` | string | required | Command to run the agent |
| `agents[].post_session` | string | `keep` | What to do after agent finishes: `keep` or `auto-pr` |
| `agents[].args_templates` | map | — | Go templates for action-specific args |
| `default_agent` | string | — | Default agent for actions |
| `notifications.desktop.enabled` | bool | `false` | Enable desktop notifications |
| `notifications.desktop.events` | list | all | Event types to notify on |
| `dashboard.items_per_repo` | int | `3` | Items shown per repo before expanding |
| `acp.socket` | string | `/tmp/ai-mux.sock` | Unix socket path |

### Agent Template Variables

Templates in `args_templates` have access to:

| Variable | Description |
|----------|-------------|
| `{{.Item.Number}}` | Issue/PR number |
| `{{.Item.Title}}` | Issue/PR title |
| `{{.Item.Body}}` | Issue/PR description |
| `{{.Item.URL}}` | GitHub URL |
| `{{.Item.Author}}` | Author username |
| `{{.Item.State}}` | State (open, closed, merged) |
| `{{.Item.HeadBranch}}` | PR head branch (PRs only) |
| `{{.Repo}}` | Repository name (owner/repo) |
| `{{.RepoPath}}` | Local repository path |
| `{{.Worktree}}` | Worktree path for this action |

### Worktree Isolation

Every agent action runs in an isolated git worktree at `<repo-path>/.worktrees/<action>-<number>`. This allows multiple agent sessions to run in parallel without interfering with each other or the current checkout.

Post-session behavior per agent:
- **`keep`** — worktree stays on disk after the agent finishes
- **`auto-pr`** — commits changes, pushes, creates a draft PR, then removes the worktree

Worktrees with no changes are always cleaned up automatically.

## Development

```sh
# Build
make build

# Run tests
make test

# Run tests with coverage
make coverage

# Format code
make fmt

# Lint
make lint

# Clean build artifacts
make clean

# Run integration tests
make integration-test
```

### Project Structure

```
cmd/ai-mux/          CLI entrypoint and cobra commands
internal/
  acp/               ACP agent (JSON-RPC IDE integration)
  action/            Action interface, registry, and implementations
    agent/           AI agent runner with template args
    assign/          Self-assignment action
    browser/         Open-in-browser action
  config/            Configuration loading and validation
  daemon/            Daemon core, client handling, PID management
  event/             Event types and channel-based event bus
  notifier/          Notification interface and implementations
    desktop/         Desktop notifications (notify-send on Linux, osascript on macOS)
    tui/             TUI badge counter
  poller/            GitHub polling orchestrator
  protocol/          Transport/connection interfaces and message types
    jsonlines/       JSON lines over Unix socket implementation
  provider/          Provider interface and implementations
    github/          GitHub provider (go-github)
    mock/            Mock provider for tests
  session/           Session lifecycle, persistence, and tmux management
  store/             Store interface and state types
    jsonfile/        JSON file store with atomic writes
  tui/               Terminal UI
    attach/          Single-item focused view with markdown rendering
    dashboard/       Multi-repo dashboard with tabs and session management
  worktree/          Git worktree management and post-session handlers
```

### Key Interfaces

- **`provider.Provider`** — abstracts GitHub API (extensible to GitLab, etc.)
- **`store.Store`** — state persistence (items, sessions, worktrees, poll times)
- **`protocol.Transport`** — client/server transport (swappable: JSON lines, gRPC)
- **`action.Action`** — executable actions with a registry for lookup
- **`notifier.Notifier`** — event notification channels

See [Design Document](docs/plans/2026-06-05-ai-mux-design.md) for full architecture details.

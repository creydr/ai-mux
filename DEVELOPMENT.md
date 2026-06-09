# Development

## Architecture

**ai-mux** uses a daemon/client architecture:

- **Daemon** — background process that polls GitHub, maintains state, manages sessions, and serves clients over a Unix socket
- **Dashboard** — full-screen TUI showing all watched repos with tabbed Issues/PRs/Sessions views
- **Session CLI** — list and attach to agent sessions from outside the dashboard

```
┌──────────────┐     ┌──────────────┐
│  Dashboard   │     │  Session CLI │
│   (TUI)      │     │              │
└──────┬───────┘     └──────┬───────┘
       │                    │
       └────────┬───────────┘
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

## Building & Testing

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

## Project Structure

```
cmd/ai-mux/          CLI entrypoint and cobra commands
internal/
  action/
    browser/         Open-in-browser helper
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
  worktree/          Git worktree management
```

## Key Interfaces

- **`provider.Provider`** — abstracts GitHub API (extensible to GitLab, etc.)
- **`store.Store`** — state persistence (items, sessions, worktrees, poll times)
- **`protocol.Transport`** — client/server transport (swappable: JSON lines, gRPC)
- **`notifier.Notifier`** — event notification channels

See [Design Document](docs/plans/2026-06-05-ai-mux-design.md) for full architecture details.

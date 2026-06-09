# ai-mux

A terminal-based tool for monitoring multiple GitHub repositories. Watches for new issues, PRs, and review activity with actionable integrations — spawn AI agent sessions to fix issues or review PRs directly from the dashboard.

## Requirements

- [Go](https://go.dev/) 1.26+
- [`gh`](https://cli.github.com/) CLI — GitHub authentication
- [`tmux`](https://github.com/tmux/tmux) — agent sessions
- `git` — worktree isolation
- Optional: `notify-send` (Linux) or `osascript` (macOS) for desktop notifications

## Installation

```sh
go install github.com/creydr/ai-mux/cmd/ai-mux@latest
```

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
  - name: gemini
    command: gemini

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

### Session

```sh
# List all sessions
ai-mux session list

# Attach to a running session (opens tmux)
ai-mux session attach <session-id>

# View output of a completed session
ai-mux session attach <session-id>
```

For running/pending sessions, `attach` opens the tmux session directly. For completed/failed/stopped sessions, it streams the captured output to stdout.

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
| `default_agent` | string | — | Default agent for actions |
| `notifications.desktop.enabled` | bool | `false` | Enable desktop notifications |
| `notifications.desktop.events` | list | all | Event types to notify on |
| `dashboard.items_per_repo` | int | `3` | Items shown per repo before expanding |
| `daemon.socket` | string | `/tmp/ai-mux.sock` | Unix socket path |

### Worktree Isolation

Every agent session runs in an isolated git worktree at `<repo-path>/.worktrees/<action>-<number>`. This allows multiple agent sessions to run in parallel without interfering with each other or the current checkout. Worktrees with no changes are cleaned up automatically.

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for build instructions, architecture, and project structure.

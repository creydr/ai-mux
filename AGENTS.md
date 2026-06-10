# AGENTS.md

## Always Keep in Mind

Act like a professional software developer and engineer. Adhere to architecture, naming conventions and coding standards in this codebase. If unsure, read similar files and get inspiration from the rest of the codebase. If introducing new features, make sure to cover them via unit tests and don't forget to take edge cases into account.

## Project Overview

Terminal-based tool for monitoring multiple GitHub repositories with integrated AI agent sessions. Written in Go, uses a daemon/client architecture over a Unix socket with a JSON lines protocol. The TUI is built on [charmbracelet/bubbletea v2](https://github.com/charmbracelet/bubbletea). Agent sessions run in isolated git worktrees inside tmux.

Flow: GitHub polling → daemon state → protocol messages → dashboard/CLI.

See [DEVELOPMENT.md](DEVELOPMENT.md) for architecture, project structure, and key interfaces.

## Testing Strategy

Before committing, test locally following the table below:

| If changed | Target | Description |
|-----------|--------|-------------|
| `*.go` files | `make test` | Unit tests |
| `*.go` files | `make fmt` | Format with gofmt and goimports |
| Any files | `make lint` | Linting |

## Key Patterns

- **Daemon/client protocol:** Clients communicate with the daemon via `protocol.Conn` over a Unix socket. Use `protocol.SendRequest()` for request-response commands (includes a 30s timeout). Use raw `conn.Receive()` only for long-polling (event listeners, output streams) where blocking is intentional.
- **Bubbletea TUI structure:** Each TUI package (dashboard, attach) follows the Model/Update/View pattern. Files are split by concern: `model.go` (state + Init/Update/View), `commands.go` (tea.Cmd functions), `messages.go` (message types), `keys.go` (key handling), `styles.go` (lipgloss styles).
- **Shared TUI types:** Common message types live in `internal/tui/` (e.g. `tui.ErrMsg`). Package-specific messages stay in their own `messages.go`.
- **Worktree isolation:** Every agent session gets its own git worktree at `<repo-path>/.worktrees/<action>-<number>`. This allows parallel sessions without conflicts. Worktrees with no changes are cleaned up automatically.
- **Session lifecycle:** Sessions are managed by the daemon, persisted to `~/.ai-mux/sessions.json`, and survive daemon restarts. The daemon reconciles persisted sessions with live tmux state on startup.
- **Provider abstraction:** The `provider.Provider` interface abstracts GitHub API access. Use `provider/mock` for tests.
- **Error responses:** Protocol error messages use `protocol.MsgError` type. Parse them with `protocol.ParseErrorPayload()`.

## Code Conventions

- Format Go files with `gofmt` and organize imports with `goimports` (local prefix: `github.com/creydr/ai-mux`).

## Boundaries

### Always Do

- Run `make test` and `make fmt` before considering any change complete
- Run `make lint` before commits
- Read existing code in the package you're modifying to match patterns

### Ask First

- Changes to the protocol message types (`internal/protocol/`)
- Adding new dependencies
- Changes to the daemon's client handling (`internal/daemon/`)
- Modifying the session lifecycle or persistence format

### Never Do

- Commit secrets, API keys, or credentials
- Delete files without explicit user approval
- Force push to main/master branch
- Skip tests or formatting

## Important Documentation

Read these files to understand the project setup, conventions, and development workflow:

- [README.md](README.md) — setup, configuration, CLI usage, keyboard shortcuts
- [DEVELOPMENT.md](DEVELOPMENT.md) — architecture, building, project structure, key interfaces

After implementing a feature or making significant changes, check whether these docs need updating.

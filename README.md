# ai-mux

A terminal-based tool for monitoring multiple GitHub repositories. Watches for new issues, PRs, and review activity with actionable integrations — spawn AI agent sessions to fix issues or review PRs, and push diffs to your IDE via ACP.

## Architecture

**ai-mux** uses a daemon/client architecture:

- **Daemon** — background process that polls GitHub, maintains state, and serves clients over a Unix socket
- **Dashboard** — full-screen TUI showing all watched repos with tabbed Issues/PRs/Activity views
- **Attach** — focused TUI for a single issue or PR, usable in a dedicated tmux pane or IDE terminal
- **ACP Agent** — IDE integration via Agent Client Protocol for viewing diffs and running agent sessions

## Build

```sh
make build
```

## Run

```sh
# Start the daemon
bin/ai-mux daemon start

# Open the dashboard
bin/ai-mux dashboard

# Focus on a specific PR in another terminal
bin/ai-mux attach pr/owner/repo/123

# Check version
bin/ai-mux version
```

## Configuration

Config lives at `~/.config/ai-mux/config.yaml`:

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
    post_session: auto-pr
    args_templates:
      fix_issue: "--print 'Fix issue #{{.Number}}: {{.Title}}'"
      review_pr: "--print 'Review PR #{{.Number}}'"

default_agent: claude

notifications:
  desktop:
    enabled: false
    events:
      - review_requested
      - review_received
```

## Test

```sh
make test
```

## Development

See [Design Document](docs/plans/2026-06-05-ai-mux-design.md) for full architecture details.

# Jira Integration Design

## Overview

Add a Jira items overview to the dashboard alongside Issues, Pull Requests, and
Sessions. Users configure a JQL filter in the config file. Jira items can be
browsed, viewed in detail, and used to spawn agent sessions — with the extra step
of selecting which configured repository to create the worktree in.

## Data Model

### JiraItem

```go
type JiraItem struct {
    Key         string    // "PROJ-123"
    Summary     string
    Description string
    Status      string    // "To Do", "In Progress", etc.
    Priority    string    // "High", "Medium", etc.
    Type        string    // "Bug", "Story", "Task", etc.
    Assignee    string
    Reporter    string
    Labels      []string
    URL         string    // Browser URL extracted from acli output
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### JiraComment

```go
type JiraComment struct {
    Author    string
    Body      string
    CreatedAt time.Time
}
```

### Session Changes

The `Session` struct gains an `ItemKey string` field for Jira items (since Jira
keys are strings like `PROJ-123`, not integers). `ItemNumber` remains for GitHub
items. `ItemType` gets a new value `"jira"`.

## Configuration

```yaml
jira:
  jql: "assignee = currentUser() AND resolution = Unresolved"
  orderBy: "priority DESC, updated DESC"
  maxResults: 50
```

- The entire `jira` block is optional. If absent, the Jira tab does not appear.
- `jql` is required when the block is present.
- `orderBy` is optional (omitted = Jira default ordering).
- `maxResults` defaults to 50 if not set. No upper cap.

## Tab System

Tabs become dynamic based on config:

- Without Jira: `Issues | PRs | Sessions`
- With Jira: `Issues | PRs | Jira | Sessions`

## Jira Tab — List View

- Flat list (no repo grouping since items come from one JQL query).
- Full terminal width (no side panel — same as Sessions tab).
- Each row: `  PROJ-123  Summary text...   [In Progress] [High]`
- Session badges appear if a session exists for the Jira item.
- `[more...]` at the bottom loads the next page of results.
- Cursor navigation, Enter for detail view, `a` to spawn, `o` to open browser.

## Detail View

Extends the existing attach model to support Jira items:

```
┌─ PROJ-123: Fix login timeout ─────────────────────────┐
│ Status: In Progress    Priority: High    Type: Bug     │
│ Assignee: jdoe         Reporter: asmith                │
│ Labels: backend, auth                                  │
├────────────────────────────────────────────────────────┤
│ Description (markdown rendered)                        │
├────────────────────────────────────────────────────────┤
│ Comments                                               │
│ asmith · 2h ago                                        │
│ This started after the last deploy...                  │
└────────────────────────────────────────────────────────┘
```

Keybindings: `a` spawn (repo picker first), `o` open browser, `t` attach
session, `r` refresh, `Esc` back.

## Session Spawning Flow

```
User presses 'a' on Jira item
  → Repo Picker (skip if only 1 repo configured)
  → Agent Picker (skip if defaultAgent set)
  → Worktree Choice (only if worktree already exists)
  → Session spawned in tmux
```

### Repo Picker

A modal matching the agent picker style. Lists repos from config (`owner/repo`).
Arrow keys + Enter to select.

### Worktree Naming

Pattern: `jira-<agent>-<JIRA-KEY>` (e.g., `jira-claude-PROJ-123`).
Branch: `ai-mux/jira-claude-PROJ-123`.
Created in the selected repo's local path under `.worktrees/`.

## Daemon — Jira Polling

Runs alongside GitHub polling on the same `pollInterval`:

- Shells out to `acli jira issue list --jql "<jql> ORDER BY <orderBy>" --limit <maxResults> --output json`
- Parses JSON into `[]JiraItem`
- Stores in memory, emits events to connected dashboards

If `acli` fails (not installed, auth expired, network), the daemon logs the
error, keeps serving stale data, and the dashboard shows a status message.

### New Protocol Messages

| Message              | Purpose                                          |
|----------------------|--------------------------------------------------|
| `MsgListJiraItems`   | Return cached items (with offset/limit for pagination) |
| `MsgGetJiraItem`     | Fetch full details via `acli jira issue view`    |
| `MsgGetJiraComments` | Fetch comments via `acli jira issue comments`    |

## Data Fetching via acli

All Jira data is fetched by shelling out to the `acli` CLI tool:

- **List:** `acli jira issue list --jql "..." --output json`
- **Detail:** `acli jira issue view <KEY> --output json`
- **Comments:** `acli jira issue comments <KEY> --output json`

Auth is handled by `acli` — no credentials in the ai-mux config.

## Resilience

- Jira polling failures are logged and retried on the next interval.
- Dashboard shows stale data with a status indicator on fetch failure.
- If Jira is not configured, all Jira-related code paths are skipped.
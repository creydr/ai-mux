# ai-mux Landing Page — Design Document

## Overview

A single-page static landing page for ai-mux, hosted via GitHub Pages. Modern developer-tool aesthetic (dark theme, blue/cyan gradient accents, clean sans-serif typography). Showcases the tool with GIF recordings of the TUI in action.

## Technical Setup

- **Hosting:** GitHub Pages, deployed via `actions/deploy-pages` on push to `main`
- **Stack:** Pure HTML + CSS + vanilla JS (smooth scroll, copy button). No framework, no build step.
- **Directory:** `/website` on `main` branch

### Color Palette

- Background: `#0a0a0f`
- Surface/card: `#141420`
- Text primary: `#e4e4e7`
- Text muted: `#a1a1aa`
- Accent gradient: `#3b82f6` (blue) → `#06b6d4` (cyan)
- Glow/border: `rgba(59, 130, 246, 0.3)`

### Typography

- Headings: Inter (Google Fonts), bold
- Body: Inter, regular/light
- Code: JetBrains Mono (Google Fonts)

### File Structure

```
website/
  index.html
  style.css
  recordings/           # GIF files go here
  vhs/                  # .tape scripts for generating GIFs
.github/workflows/
  deploy-website.yaml   # GitHub Pages deployment
```

## Page Sections

### 1. Hero

- Large bold tagline (e.g., "Your AI-powered command center for GitHub & Jira")
- One-liner subtitle in muted text
- Styled install command with copy button: `go install github.com/creydr/ai-mux/cmd/ai-mux@latest`
- Two CTA buttons: "Get Started" (scrolls to install) + "GitHub" (repo link)
- Large GIF placeholder below — terminal-window frame with blue/cyan glow, showing the main dashboard
- Subtle radial gradient background + faint dot grid pattern

### 2. Features (2x2 Grid)

Four cards with SVG icon, title, short description. Subtle border, hover glow.

1. **Multi-Repo Dashboard** — Monitor issues, PRs, and Jira items across all your repositories in one tabbed interface.
2. **AI Agent Spawning** — Launch Claude, Gemini, or any AI agent directly from an issue or PR — one keystroke.
3. **Isolated Worktrees** — Each agent session gets its own git worktree. Run multiple agents in parallel without conflicts.
4. **Jira Integration** — Pull in Jira boards alongside GitHub. Search, filter, and drill into details without leaving the terminal.

### 3. How It Works (Zigzag Steps)

Three steps, alternating text/GIF sides:

1. **Configure your repos** — Point ai-mux at your GitHub repos and Jira boards with a simple YAML config. *GIF: config editing + daemon start*
2. **Browse everything in one place** — Switch between Issues, PRs, Jira items, and Sessions with tab navigation. Search and filter instantly. *GIF: tabbing, searching, detail view*
3. **Spawn an AI agent** — Press Enter on any issue, pick your agent, and it starts working in an isolated worktree. Attach to watch it live. *GIF: spawning agent, attaching*

### 4. Installation / Quick Start

- Heading: "Get Started in 30 Seconds"
- Step 1: `go install` command in styled code block
- Step 2: Minimal YAML config example (syntax highlighted)
- Step 3: `ai-mux daemon start && ai-mux dashboard`
- Link: "View full docs on GitHub"

### 5. Footer

Minimal single line: GitHub link, "Built with Go & Bubbletea"

## GitHub Action

Workflow at `.github/workflows/deploy-website.yaml`:
- Triggers on push to `main` when `website/**` changes
- Uses `actions/upload-pages-artifact` + `actions/deploy-pages`
- Requires Pages enabled in repo settings with source "GitHub Actions"

## GIF Recordings

VHS `.tape` scripts will be provided in `website/vhs/` for generating four GIFs:

1. `dashboard-overview.tape` — Hero GIF: navigating the full dashboard
2. `config-and-start.tape` — How It Works step 1: config + daemon start
3. `browsing.tape` — How It Works step 2: tabs, search, detail view
4. `spawn-agent.tape` — How It Works step 3: spawning and attaching to an agent

User runs `vhs <file>.tape` to produce GIFs into `website/recordings/`.
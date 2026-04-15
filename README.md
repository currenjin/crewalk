# crewalk

TUI dashboard that visualizes Claude Code work sessions as characters walking through workflow stages.

Each ticket becomes a character. Each workflow stage is a room. You watch your crew walk from planning to done.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ 🏢 ROOUTY WORK DASHBOARD                                    2026-04-15 10:23 │
╰──────────────────────────────────────────────────────────────────────────────╯
╭──────────╮╭──────────╮╭──────────╮╭──────────╮╭──────────╮╭──────────╮
│ PLANNING ││BRANCHING ││  CODING  ││REVIEWING ││ PUSH/PR  ││   DONE   │
│          ││          ││          ││          ││          ││          │
│          ││ RP-1234  ││          ││ RP-5678  ││          ││          │
│          ││   🧑     ││          ││   🧑     ││          ││          │
│          ││ working..││          ││ reviewing││          ││          │
╰──────────╯╰──────────╯╰──────────╯╰──────────╯╰──────────╯╰──────────╯
          RP-9999🚶

[n] new ticket  [ctrl+c] quit
```

## Prerequisites

- [Go](https://go.dev/) 1.21+
- [Claude Code](https://claude.ai/code) (`claude` command available in PATH)
- git

## Installation

```bash
go install github.com/currenjin/crewalk/cmd/crewalk@latest
```

## Usage

Run from inside your git project:

```bash
cd /path/to/your/project
crewalk
```

crewalk auto-detects the project root by walking up to find `.git`. No configuration needed for basic use.

### Key bindings

| Key | Action |
|-----|--------|
| `n` | Open new ticket input |
| `Enter` | Confirm input |
| `Esc` | Cancel input |
| `Ctrl+C` | Quit |

### Starting a ticket

1. Press `n`
2. Type a ticket ID (e.g. `RP-1234`)
3. Press `Enter`

crewalk creates a git worktree, spawns a Claude Code session, and injects `/work RP-1234` to start the work session. The character appears and begins walking through stages as Claude works.

### Answering questions

When Claude needs input, the character pauses and a question box appears at the bottom of the screen. Type your answer and press `Enter`. If multiple tickets are asking questions simultaneously, they queue up — one at a time, in order.

## Configuration

crewalk works without any config file. To override defaults, create `~/.config/crewalk/config.toml`:

```toml
[project]
path = "/path/to/your/project"
worktree_base = "/path/to/worktrees"

[claude]
command = "claude"
args = ["--dangerously-skip-permissions"]
```

| Field | Default | Description |
|-------|---------|-------------|
| `project.path` | auto-detected from `.git` | Absolute path to your project root |
| `project.worktree_base` | `../worktree` relative to project root | Where git worktrees are created |
| `claude.command` | `claude` | Claude Code binary name or path |
| `claude.args` | `["--dangerously-skip-permissions"]` | Arguments passed to every Claude session |

## How it works

- Each ticket gets its own git worktree under `worktree_base/feature/<ticket-id>`
- A Claude Code process runs in that worktree with stdin/stdout piped
- crewalk watches `~/.claude/projects/*/*.jsonl` transcripts to detect phase transitions and questions
- Phase changes animate the character walking to the next room
- On quit, all sessions are gracefully shut down and worktrees are removed

## Workflow stages

| Stage | Description |
|-------|-------------|
| PLANNING | Claude is reading the ticket and planning the approach |
| BRANCHING | Creating the git branch |
| CODING | Implementing the changes |
| REVIEWING | Running tests, reviewing the diff |
| PUSH/PR | Pushing and opening a pull request |
| DONE | Work complete |

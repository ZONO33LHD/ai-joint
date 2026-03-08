# ai-joint

A multi-session manager for [Claude Code](https://claude.ai/code) — no tmux required.

Spin up isolated Claude Code sessions per project, watch their output side-by-side in a terminal TUI, and track every custom command, rules file, and sub-agent invocation in a persistent SQLite history.

```
┌──ai-joint──────────────────────────────────────┐
│ ┌─ Sessions ────────┐ ┌─ api-refactor ─────────┐│
│ │ ● api-refactor    │ │                         ││
│ │   busy · 14:23    │ │  Claude Code output     ││
│ │                   │ │  streams here…          ││
│ │ ○ ui-components   │ │                         ││
│ │   idle · 14:15    │ └─────────────────────────┘│
│ │                   │                            │
│ ├─ Activity ────────┤                            │
│ │ cmd   /review  14:23                           │
│ │ rule  CLAUDE.md 14:20                          │
│ │ agent subagent 14:15                           │
│ └───────────────────┘                            │
└────────────────────────────────────────────────┘
```

## Features

- **No tmux** — pure Go PTY via [creack/pty](https://github.com/creack/pty)
- **TUI dashboard** — split-pane view built with [tview](https://github.com/rivo/tview); left panel shows sessions + activity history, right panel streams the selected session's output
- **Activity tracking** — records custom commands (`/review`), rules files (`CLAUDE.md`), and sub-agent names via Claude Code hooks
- **Persistent state** — SQLite database (pure Go, no cgo) at `~/.local/share/ai-joint/ai-joint.db`
- **Session states** — `● busy` (yellow) · `○ idle` (green) · `✓ done` (grey)

## Requirements

- Go 1.26+
- Claude Code (`claude` binary in `$PATH`)

## Installation

```bash
git clone https://github.com/shunsuke/ai-joint
cd ai-joint
go build -o ~/.local/bin/aj .
```

`~/.local/bin` is already on `$PATH` for most setups. Verify with:

```bash
aj --help
```

If `command not found`, add the directory to your `$PATH`:

```bash
# Add to ~/.zshrc or ~/.bashrc
export PATH="$HOME/.local/bin:$PATH"

source ~/.zshrc   # or ~/.bashrc
```

## Usage

### Launch a session

Start a new Claude Code session in a PTY. The session is registered in the database and its output is forwarded to your terminal.

```bash
# Launch in the current directory
aj launch my-task

# Launch in a specific directory
aj launch api-refactor --dir ~/projects/api

# Pass extra environment variables
aj launch ui-work --dir ~/projects/ui --env ANTHROPIC_LOG=debug

# Use a custom claude binary path
aj launch my-task --cc /usr/local/bin/claude
```

Sessions keep running in the foreground. Open another terminal and use `aj dashboard` to watch them all at once.

### Open the TUI dashboard

```bash
aj dashboard
```

**Keyboard shortcuts:**

| Key | Action |
|-----|--------|
| `Tab` | Switch focus between the session list and the output panel |
| `↑` / `↓` | Navigate sessions |
| `Enter` | Select a session (loads its output on the right) |
| `q` | Quit |

### List sessions

```bash
# Table output
aj ls

# JSON (suitable for scripting / jq)
aj ls --json

# Suppress the header row
aj ls --no-header
```

Example output:

```
NAME           STATE  DIR                       UPDATED
api-refactor   busy   /home/user/projects/api   2026-03-08 14:23:01
ui-components  idle   /home/user/projects/ui    2026-03-08 14:15:44
```

### Remove a session

Deletes the session record from the database. Does **not** kill the underlying process if it is still running.

```bash
aj kill api-refactor
```

### Process a Claude Code hook event

`aj hook` reads a JSON event from stdin and records it as an activity. Wire it up in your Claude Code hook configuration so activity appears automatically in the dashboard.

```bash
echo '{"event":"PreToolUse","session_id":"<id>","custom_command":"/review"}' | aj hook
```

## Claude Code Hook Integration

Add the following to your Claude Code settings (`.claude/settings.json` or `settings.local.json`) to track commands, rules files, and agents automatically:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "aj hook"
          }
        ]
      }
    ]
  }
}
```

Claude Code will pipe a JSON payload to `aj hook` on every tool invocation. The payload is parsed for the following fields:

| Field | Activity kind | Example value |
|-------|--------------|---------------|
| `custom_command` | `cmd` | `/review` |
| `rules_file` | `rule` | `CLAUDE.md` |
| `agent_name` | `agent` | `subagent` |

Unrecognised fields are silently ignored; the hook exits with code 0 so Claude Code is never blocked.

## Data storage

All state is stored in a single SQLite file:

```
~/.local/share/ai-joint/ai-joint.db
```

Schema:

```sql
CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    dir        TEXT NOT NULL,
    state      TEXT NOT NULL DEFAULT 'idle',  -- 'busy' | 'idle' | 'done'
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE activities (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL,
    kind        TEXT NOT NULL,   -- 'cmd' | 'rule' | 'agent'
    value       TEXT NOT NULL,
    occurred_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);
```

## Project structure

```
ai-joint/
├── main.go
├── cmd/
│   ├── root.go        # cobra root ("aj")
│   ├── launch.go      # aj launch
│   ├── dashboard.go   # aj dashboard
│   ├── ls.go          # aj ls
│   ├── hook.go        # aj hook
│   └── kill.go        # aj kill
└── internal/
    ├── session/
    │   ├── session.go  # Session type, state constants
    │   └── manager.go  # in-memory session registry, synced to SQLite
    ├── tui/
    │   ├── app.go      # tview Application, flex layout
    │   ├── sidebar.go  # left panel: session list + activity feed
    │   └── viewport.go # right panel: PTY output for selected session
    ├── tracker/
    │   └── tracker.go  # parses hook JSON, writes activity rows
    ├── store/
    │   ├── store.go    # SQLite CRUD
    │   └── schema.go   # embedded CREATE TABLE statements
    └── pty/
        └── pty.go      # spawns claude in a PTY, streams I/O
```

## Dependencies

| Package | Purpose |
|---------|---------|
| [rivo/tview](https://github.com/rivo/tview) | Terminal UI framework |
| [gdamore/tcell/v2](https://github.com/gdamore/tcell) | Low-level terminal cells (tview dependency) |
| [spf13/cobra](https://github.com/spf13/cobra) | CLI framework |
| [creack/pty](https://github.com/creack/pty) | Pure Go PTY |
| [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | Pure Go SQLite (no cgo) |

## License

MIT


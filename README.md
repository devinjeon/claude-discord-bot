# claude-discord-bot

A system for remotely monitoring and controlling Claude Code tmux sessions via Discord.

## Components

This project consists of two services:

| Service | tmux session | Role |
|---------|-------------|------|
| claude-channel | `claude-channel` | Runs Claude Code with the Discord channel plugin |
| claude-bot | `claude-bot` | Monitors the claude-channel session and relays interactions via Discord |

claude-channel is the actual Claude Code agent. claude-bot watches its tmux session and relays situations requiring user intervention (choices, text input) to Discord.

## Features

### Discord commands
- `/claude-now` — Show current terminal output of the Claude tmux session
- `/claude-screenshot` — Take a system screenshot and send it
- `/claude-restart` — Restart the Claude tmux session (LaunchAgent auto-recovers)
- `/claude-usage` — Check Claude Code usage stats

### Automatic features
- Choice detection — When a selection prompt appears in tmux, it sends emoji buttons to Discord
- Text input relay — When Claude requests text input, you can type it via Discord message
- Restart notification — Automatically notifies Discord when a Claude session starts (SessionStart hook)

## How it works

```
┌─────────────┐      ┌─────────────┐      ┌─────────────────┐
│   Discord    │◄────►│  claude-bot  │─────►│  claude-channel  │
│   (user)     │      │  (Go bot)   │ tmux │  (Claude Code)   │
└─────────────┘      └─────────────┘      └─────────────────┘
       ▲                                          │
       └──────────────────────────────────────────┘
                   Direct chat via Discord plugin
```

1. `run-channel.sh` runs Claude Code with Discord plugin in a tmux session
2. `run-bot.sh` runs the Go bot in a separate tmux session
3. The bot polls the claude-channel session for selection prompts (default: every 5 seconds)
4. Detected choices are sent to the Discord channel with emoji reactions
5. When the user clicks an emoji, the corresponding selection is sent as key input to tmux
6. Both processes are managed by macOS LaunchAgent for automatic start/restart

## Prerequisites

### Required software

| Software | Purpose | Install |
|----------|---------|---------|
| Go | Build the bot binary | `brew install go` |
| tmux | Session management | `brew install tmux` |
| Claude Code | The AI agent | `brew install claude-code` |
| jq | JSON processing in install script | `brew install jq` |

All of these must be available in your `$PATH` at install time. The installer captures your current shell environment (PATH, SHELL, binary locations) and uses it for the LaunchAgent configuration.

### Discord plugin setup

The Discord plugin (`plugin:discord`) must be configured in Claude Code before installing this bot.
The plugin creates the following files automatically, which claude-bot reads at startup:

| File | Created when | Contents |
|------|-------------|----------|
| `~/.claude/channels/discord/.env` | Running `/discord:configure` and entering bot token | `DISCORD_BOT_TOKEN=...` |
| `~/.claude/channels/discord/access.json` | Pairing a channel | Allowed channel IDs, user policies, etc. |

You do not need to manually create `.env` or set channel/guild IDs.

## Install

```bash
make install
```

This runs `deploy/install.sh`, which performs the following steps:

1. Verify prerequisites (go, tmux, claude, jq in PATH)
2. Capture current shell environment (HOME, SHELL, PATH, binary paths) into `deploy/env.generated.sh`
3. Build the `claude-bot` Go binary
4. Stop existing services if running
5. Generate LaunchAgent plist files with the captured PATH
6. Install Claude Code SessionStart hook — when the Claude session starts, `discord-restart-notify.sh` sends a notification to the Discord channel so you know the session is back up
7. Load LaunchAgents

The install is idempotent — running it again updates everything cleanly.

After installing, apply changes by reinstalling:

```bash
make install
```

## Uninstall

```bash
make uninstall
```

This runs `deploy/uninstall.sh`, which:

1. Stops and removes LaunchAgents
2. Kills tmux sessions (`claude-bot`, `claude-channel`)
3. Removes the SessionStart hook from `~/.claude/settings.json`

The uninstall is also idempotent.

## Configuration

There are three customization files, none of which are tracked by git:

| File | Purpose | Created by |
|------|---------|------------|
| `deploy/claude-channel/channel.env` | Claude CLI flags and channel settings | User (copy from `channel.env.example`) |
| `deploy/claude-channel/pre-run.sh` | Shell environment setup before Claude starts | User (copy from `pre-run.sh.example`) |
| `deploy/env.generated.sh` | Captured shell environment at install time | `install.sh` (auto-generated, do not edit) |

After editing any config file, run `make install` to apply.

### channel.env — Claude CLI options

Controls how Claude Code is launched. Create it from the example:

```bash
cp deploy/claude-channel/channel.env.example deploy/claude-channel/channel.env
```

Available variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `CLAUDE_CHANNELS` | `plugin:discord@claude-plugins-official` | Discord plugin channel identifier |
| `CLAUDE_FLAGS` | *(empty)* | CLI flags passed to `claude` (e.g. `--debug`, `--dangerously-skip-permissions`) |
| `CLAUDE_EXTRA_ARGS` | *(empty)* | Additional arguments appended after `CLAUDE_FLAGS` |

Examples:

```bash
# Enable debug logging
CLAUDE_FLAGS="--debug"

# Enable debug + skip permission prompts (use with caution)
CLAUDE_FLAGS="--debug --dangerously-skip-permissions"

# Use a different Discord plugin channel
CLAUDE_CHANNELS="plugin:discord@my-custom-plugin"

# Pass extra arguments
CLAUDE_EXTRA_ARGS="--verbose"
```

### pre-run.sh — Runtime environment

Sourced inside the tmux session before Claude starts. Both `claude-channel` and `claude-bot` sessions source this file. Optional — if the file doesn't exist, it is skipped.

Create it from the example:

```bash
cp deploy/claude-channel/pre-run.sh.example deploy/claude-channel/pre-run.sh
```

Example:

```bash
source "$HOME/.zshrc" 2>/dev/null
export PATH="$HOME/.local/bin:$PATH"
```

See `pre-run.sh.example` for more examples (nvm, pyenv, cargo, etc.).

### Bot settings via environment variables

These are read from `~/.claude/channels/discord/.env` or system environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DISCORD_BOT_TOKEN` | *(required)* | Discord bot token |
| `DISCORD_CHANNEL_ID` | Auto-detected from `access.json` | Target Discord channel ID |
| `DISCORD_GUILD_ID` | Auto-detected via Discord API | Discord server (guild) ID |
| `POLL_INTERVAL` | `5` | Polling interval in seconds for checking tmux prompts |

Config resolution order:

| Item | 1st | 2nd | 3rd |
|------|-----|-----|-----|
| Bot Token | env var | `.env` | — |
| Channel ID | env var | `.env` | `access.json` `groups` key |
| Guild ID | env var | `.env` | Discord API auto-lookup |

To change the polling interval, add to `~/.claude/channels/discord/.env`:

```bash
echo "POLL_INTERVAL=10" >> ~/.claude/channels/discord/.env
```

Then restart the bot:

```bash
make install
```

## Development

```bash
# Build only
make build

# Run directly (requires DISCORD_BOT_TOKEN)
make run

# Run tests
make test

# Run tests with race detector
make test-race

# Test coverage summary
make coverage

# HTML coverage report
make coverage-html

# Vet + race tests
make check
```

## Project structure

```
.
├── cmd/claude-bot/              # Bot entrypoint
│   └── main.go
├── internal/
│   ├── bot/                     # Discord bot core logic
│   │   ├── bot.go               # Init, handler registration, run
│   │   ├── commands.go          # Slash/message command handlers
│   │   ├── polling.go           # tmux choice polling + text input detection
│   │   └── state.go             # Interaction state management
│   ├── tmux/                    # tmux operation utilities
│   │   └── tmux.go
│   └── imaging/                 # Screenshot + image processing
│       └── screenshot.go
├── deploy/
│   ├── bot/
│   │   └── run-bot.sh           # Bot launcher script
│   ├── claude-channel/
│   │   ├── run-channel.sh       # Claude launcher script
│   │   ├── pre-run.sh.example   # Pre-run environment template
│   │   ├── channel.env.example  # Channel config template
│   │   └── discord-restart-notify.sh
│   ├── install.sh
│   └── uninstall.sh
├── .github/workflows/
│   └── release.yml              # GoReleaser GitHub Actions
├── .goreleaser.yml
├── Makefile
├── go.mod
└── go.sum
```

## Release

Push a tag to trigger GoReleaser via GitHub Actions:

```bash
git tag v0.1.0
git push origin v0.1.0
```

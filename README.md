# claude-discord-bot

A companion bot for [Claude Channel](https://code.claude.com/docs/en/channels) that adds full terminal monitoring and interactive control to your Discord-based Claude Code workflow.

## The problem: Claude Channel alone isn't enough

[Claude Channel](https://code.claude.com/docs/en/channels) lets you chat with Claude from Discord. But chatting is only half the story.

### Selection prompts block silently

Claude frequently asks you to pick from a numbered list — which file to edit, which test to run, which approach to take. These prompts render in the terminal and wait for a keypress. The Discord plugin has no way to detect or relay them. Without someone watching the terminal, Claude just sits there indefinitely.

### Permission and confirmation prompts need a physical keypress

Claude Code asks for permission before running tools, editing files, or executing commands. These prompts show "Enter to confirm" / "Esc to go back" in the terminal and block until someone presses a key. The Discord plugin has no way to relay them. The usual workaround is launching with `--dangerously-skip-permissions`, which bypasses all safety checks — not ideal for unattended sessions where you still want control over what gets approved.

### The terminal is invisible from Discord

The Discord plugin relays Claude's chat messages, but not the terminal itself. You can't see build output, error logs, or what's currently on screen. When Claude goes quiet, there's no way to tell if it's working, waiting for input, or crashed.

### No remote control

Need to restart a stuck session? Check usage stats? Take a screenshot? Send a `/model` or `/usage` command? None of this is possible through the chat plugin.

## How claude-bot solves this

claude-bot runs alongside claude-channel and watches the terminal via tmux. It bridges the gap between Discord and everything the chat plugin can't reach.

```
Single-channel mode:

  Discord  <--->  claude-bot  --->  claude-channel
   (you)          (monitor)   tmux  (Claude Code)
     ^                                    |
     +------------------------------------+
           Direct chat via Claude Channel

Multi-channel mode:

  #home         <-->  claude-channel-home        (~/           )
  #my-project   <-->  claude-channel-my-project  (~/my-project )
  #another      <-->  claude-channel-another     (~/another    )
        \               /
         claude-bot (1 process, routes by channel ID)
```

- Runs multiple Claude sessions, each in its own working directory, each linked to a separate Discord channel -- one bot process routes all commands
- Detects permission and confirmation prompts and forwards them to Discord with Enter/Esc reactions -- approve or reject from your phone without `--dangerously-skip-permissions`
- Detects selection prompts and sends them as numbered emoji reactions -- tap to choose
- Lets you view terminal output, take screenshots, restart sessions, and send arbitrary keystrokes
- Runs as a macOS service via LaunchAgents -- starts on login, survives reboots, auto-restarts on crash
- Notifies the correct channel when a session restarts

## Discord commands

| Command | What it does |
|---------|-------------|
| `/claude-now` | Show current terminal output |
| `/claude-screenshot` | Take a screenshot of the desktop and send it |
| `/claude-restart` | Fully reset and restart the Claude session (LaunchAgent auto-recovers) |
| `/claude-usage` | Check Claude Code usage stats |
| `/claude-export` | Run `/export` in the Claude session |
| `/claude-model` | Run `/model` in the Claude session |
| `/claude-compact` | Run `/compact` in the Claude session |
| `/claude-clear` | Run `/clear` in the Claude session |
| `/claude-skills` | Run `/skills` in the Claude session |
| `/claude-login` | Run `/login` in the Claude session |
| `/claude-logout` | Run `/logout` in the Claude session |
| `/claude-sendkey` | Send a key or text to the terminal (supports autocomplete) |

Automatic behaviors:
- Selection prompts are detected and forwarded with emoji reactions (1-9 + Esc)
- Confirmation prompts show Enter/Esc buttons
- Session restarts trigger a Discord notification

## Prerequisites

| Software | Purpose | Install |
|----------|---------|---------|
| Go | Build the bot binary | `brew install go` |
| tmux | Session management | `brew install tmux` |
| Claude Code | The AI agent | `brew install claude-code` |
| jq | JSON processing in install script | `brew install jq` |

All must be in your `$PATH` at install time.

### Discord channel pairing

Set up the [Discord channel](https://code.claude.com/docs/en/channels#discord) first. The pairing process creates the bot token and channel configuration automatically. claude-bot reads these files at startup — no additional token or environment variable setup is needed.

## Install

```bash
make install
```

This captures your shell environment, builds the bot, sets up LaunchAgents, and installs a SessionStart hook for restart notifications. The install is idempotent.

## Uninstall

```bash
make uninstall
```

Stops services, removes LaunchAgents, and cleans up the SessionStart hook.

## Configuration

Customization files (none tracked by git):

| File | Purpose | How to create |
|------|---------|--------------|
| `deploy/instances.conf` | Multi-channel instance definitions | `cp deploy/instances.conf.example deploy/instances.conf` |
| `deploy/claude-channel/channel.env` | Claude CLI flags and channel settings | `cp deploy/claude-channel/channel.env.example deploy/claude-channel/channel.env` |
| `deploy/claude-channel/pre-run.sh` | Shell environment setup sourced before Claude starts | `cp deploy/claude-channel/pre-run.sh.example deploy/claude-channel/pre-run.sh` |
| `deploy/bot/bot.env` | Bot process environment variables (e.g. `POLL_INTERVAL`) | `cp deploy/bot/bot.env.example deploy/bot/bot.env` |
| `deploy/env.generated.sh` | Shell environment snapshot | Auto-generated by `install.sh` — do not edit |

After editing any config, run `make install` to apply.

### channel.env

Controls how Claude Code is launched.

| Variable | Default | Description |
|----------|---------|-------------|
| `CLAUDE_CHANNELS` | `plugin:discord@claude-plugins-official` | Discord plugin channel identifier |
| `CLAUDE_FLAGS` | *(empty)* | CLI flags passed to `claude` (e.g. `--debug`, `--dangerously-skip-permissions`) |
| `CLAUDE_EXTRA_ARGS` | *(empty)* | Additional arguments appended after `CLAUDE_FLAGS` |

### pre-run.sh

Sourced inside the tmux session before Claude starts. Use it to set up PATH, load version managers (nvm, pyenv, etc.), or source your shell profile. See `pre-run.sh.example` for examples.

### Multi-channel setup (instances.conf)

Run multiple Claude sessions, each linked to a separate Discord channel and working directory. One bot process routes all commands based on which channel they come from.

```bash
cp deploy/instances.conf.example deploy/instances.conf
```

Edit `deploy/instances.conf` with your channel IDs:

```
# instance_name       channel_id             working_directory
home                  1234567890123456789    ~/
my-project            1234567890123456790    ~/my-project
```

Each instance gets its own tmux session (`claude-channel-<name>`), LaunchAgent, and Discord state directory (so each Claude session only listens to its own channel via `DISCORD_STATE_DIR`).

#### Adding a new channel

1. Create a text channel in your Discord server
2. Copy the channel ID (right-click the channel, "Copy Channel ID")
3. Add a line to `deploy/instances.conf`
4. Run `make install`

The install script automatically creates the per-instance `access.json` and `.env` files. No manual Discord pairing is needed for new channels -- the bot token is shared from the original pairing.

If `instances.conf` is absent, the bot falls back to single-channel mode using the original pairing config.

### bot.env

Controls bot process settings. These are injected into the bot's LaunchAgent plist as environment variables during `make install`.

```bash
cp deploy/bot/bot.env.example deploy/bot/bot.env
```

| Variable | Default | Description |
|----------|---------|-------------|
| `POLL_INTERVAL` | `15` | Seconds between tmux prompt checks |

Add any key=value pair to `bot.env` and it will be included in the bot's plist `EnvironmentVariables`. Lines starting with `#` are ignored.

### Bot settings

Bot token, channel ID, and guild ID are all auto-detected from the Discord channel pairing files. You only need to set these manually if you want to override the defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `DISCORD_BOT_TOKEN` | Auto-detected from pairing | Discord bot token |
| `DISCORD_CHANNEL_ID` | Auto-detected from `access.json` | Target Discord channel ID |
| `DISCORD_GUILD_ID` | Auto-detected via Discord API | Discord server (guild) ID |
| `POLL_INTERVAL` | `15` | Seconds between tmux prompt checks |

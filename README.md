# claude-discord-bot

A companion bot for [Claude Channel](https://code.claude.com/docs/en/channels) that adds full terminal monitoring and interactive control to your Discord-based Claude Code workflow.

## What claude-bot adds to Claude Channel

claude-bot fills in what [Claude Channel](https://code.claude.com/docs/en/channels) doesn't cover.

### In-channel prompt interaction

Claude Channel handles prompts via DM buttons. claude-bot surfaces them as in-channel emoji reactions instead — selection prompts become numbered reactions, permission prompts get Enter/Esc. No DM switching needed.

### Terminal visibility

Claude Channel only relays chat messages. Build output, error logs, and the current screen are invisible. When Claude goes silent, there's no way to tell if it's working, stuck on a prompt, or crashed. claude-bot lets you check the terminal (`/claude-now`) and take screenshots (`/claude-screenshot`) directly from Discord.

### Session management

Claude Channel doesn't manage the Claude Code process — crashes, hangs, and reboots require manual recovery. claude-bot runs as a macOS LaunchAgent daemon that auto-starts on login and auto-restarts on crash. You can restart, send slash commands (`/model`, `/usage`, `/compact`), and send keystrokes remotely via Discord.

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

### Comparison with official alternatives

Anthropic offers other remote access options — [Remote Control](https://code.claude.com/docs/en/remote-control), [Dispatch](https://claude.com/blog/dispatch-and-computer-use), and [Cloud](https://code.claude.com/docs/en/claude-code-on-the-web). Remote Control provides a full web UI but has no messenger integration — you have to check it yourself to see if Claude needs input. claude-bot pushes notifications to Discord, so you only look when needed.

| | claude-bot | Channels alone | Remote Control | Dispatch | Cloud |
|---|---|---|---|---|---|
| Notifies you when Claude needs input | Discord push | Discord push | No (must check manually) | Mobile app push | Mobile app push |
| Remote restart | `/claude-restart` + LaunchAgent auto-recovery | Not possible | Not possible | Not possible | Cloud-managed |
| Terminal visibility | `/claude-now`, screenshots | Not possible | Full web UI | Desktop UI | Web UI |
| Failure recovery | LaunchAgent auto-restart | Manual | Manual | Manual | Cloud-managed |
| Multi-project | Single config file (instances.conf) | Separate sessions manually | Separate processes or server mode | Parallel sessions from app | Multiple repos |
| Prompt handling | In-channel emoji reactions (single tap) | DM buttons | Web/app UI | Mobile/Desktop app | Web UI |

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
3. Run:

```bash
make add-channel NAME=my-project CHANNEL=123456789012345678 DIR=~/my-project
```

This adds the entry to `instances.conf`, creates per-instance config files, starts the new channel service, and restarts only the bot (to pick up the new routing). Existing channel sessions are not affected.

#### Removing a channel

```bash
make remove-channel NAME=my-project
```

This stops the channel's LaunchAgent, kills its tmux session, removes the plist and per-instance state directory, removes the entry from `instances.conf`, and restarts the bot.

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

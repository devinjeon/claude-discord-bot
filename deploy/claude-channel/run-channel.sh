#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_SNAPSHOT="$SCRIPT_DIR/../env.generated.sh"

# Load environment snapshot captured at install time
if [[ -f "$ENV_SNAPSHOT" ]]; then
    source "$ENV_SNAPSHOT"
fi

# INSTANCE_NAME and WORKING_DIR can be set via environment (LaunchAgent plist)
INSTANCE_NAME="${INSTANCE_NAME:-}"
WORKING_DIR="${WORKING_DIR:-$HOME}"

if [[ -n "$INSTANCE_NAME" ]]; then
    SESSION="claude-channel-${INSTANCE_NAME}"
else
    SESSION="claude-channel"
fi

TMUX="${INSTALL_TMUX:-$(command -v tmux)}"
CLAUDE="${INSTALL_CLAUDE:-$(command -v claude)}"
USER_SHELL="${INSTALL_SHELL:-zsh}"

# Load optional config file
CONFIG="$SCRIPT_DIR/channel.env"
if [[ -f "$CONFIG" ]]; then
    source "$CONFIG"
fi

# Customizable options via environment variables or channel.env
CLAUDE_CHANNELS="${CLAUDE_CHANNELS:-plugin:discord@claude-plugins-official}"
CLAUDE_FLAGS="${CLAUDE_FLAGS:-}"
CLAUDE_EXTRA_ARGS="${CLAUDE_EXTRA_ARGS:-}"

# Kill existing session and its child processes
if "$TMUX" has-session -t "$SESSION" 2>/dev/null; then
    for pid in $("$TMUX" list-panes -t "$SESSION" -F '#{pane_pid}' 2>/dev/null); do
        pkill -TERM -P "$pid" 2>/dev/null
        kill -TERM "$pid" 2>/dev/null
    done
    sleep 1
    "$TMUX" kill-session -t "$SESSION" 2>/dev/null
fi

# Expand ~ in WORKING_DIR
WORKING_DIR="${WORKING_DIR/#\~/$HOME}"

# Set per-instance Discord state dir so each instance has its own access.json
if [[ -n "$INSTANCE_NAME" ]]; then
    DISCORD_STATE_DIR="$HOME/.claude/channels/discord-${INSTANCE_NAME}"
else
    DISCORD_STATE_DIR="$HOME/.claude/channels/discord"
fi

# Start Claude in a new tmux session
"$TMUX" new-session -d -s "$SESSION" \
    "/bin/$USER_SHELL -c '
export HOME=$HOME
export CLAUDE_DISCORD_SESSION=1
export DISCORD_STATE_DIR=$DISCORD_STATE_DIR
# Prevent tmux from blocking nested session creation when \$TMUX is already set
unset TMUX
[[ -f \"$SCRIPT_DIR/pre-run.sh\" ]] && source \"$SCRIPT_DIR/pre-run.sh\"
cd \"$WORKING_DIR\" 2>/dev/null || true
exec $CLAUDE --channels $CLAUDE_CHANNELS $CLAUDE_FLAGS $CLAUDE_EXTRA_ARGS
'"

# Wait while session is alive (launchd monitors this script)
while "$TMUX" has-session -t "$SESSION" 2>/dev/null; do
    sleep 10
done

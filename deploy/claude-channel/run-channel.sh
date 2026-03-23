#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_SNAPSHOT="$SCRIPT_DIR/../env.generated.sh"

# Load environment snapshot captured at install time
if [[ -f "$ENV_SNAPSHOT" ]]; then
    source "$ENV_SNAPSHOT"
fi

SESSION="claude-channel"
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

# Start Claude in a new tmux session
"$TMUX" new-session -d -s "$SESSION" \
    "/bin/$USER_SHELL -c '
export HOME=$HOME
export CLAUDE_DISCORD_SESSION=1
# Prevent tmux from blocking nested session creation when \$TMUX is already set
unset TMUX
[[ -f \"$SCRIPT_DIR/pre-run.sh\" ]] && source \"$SCRIPT_DIR/pre-run.sh\"
exec $CLAUDE --channels $CLAUDE_CHANNELS $CLAUDE_FLAGS $CLAUDE_EXTRA_ARGS
'"

# Wait while session is alive (launchd monitors this script)
while "$TMUX" has-session -t "$SESSION" 2>/dev/null; do
    sleep 10
done

#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_SNAPSHOT="$SCRIPT_DIR/../env.generated.sh"

# Load environment snapshot captured at install time
if [[ -f "$ENV_SNAPSHOT" ]]; then
    source "$ENV_SNAPSHOT"
fi

SESSION="claude-bot"
TMUX="${INSTALL_TMUX:-$(command -v tmux)}"
USER_SHELL="${INSTALL_SHELL:-zsh}"
BOT="$(cd "$SCRIPT_DIR/../.." && pwd)/claude-bot"

# Kill existing session and its child processes
if "$TMUX" has-session -t "$SESSION" 2>/dev/null; then
    for pid in $("$TMUX" list-panes -t "$SESSION" -F '#{pane_pid}' 2>/dev/null); do
        pkill -TERM -P "$pid" 2>/dev/null
        kill -TERM "$pid" 2>/dev/null
    done
    sleep 1
    "$TMUX" kill-session -t "$SESSION" 2>/dev/null
fi

# Start bot in a new tmux session
"$TMUX" new-session -d -s "$SESSION" \
    "/bin/$USER_SHELL -c '
export HOME=$HOME
# Prevent tmux from blocking nested session creation when $TMUX is already set
unset TMUX
PRE_RUN=\"$SCRIPT_DIR/../claude-channel/pre-run.sh\"
[[ -f \"\$PRE_RUN\" ]] && source \"\$PRE_RUN\"
exec $BOT
'"

# Wait while session is alive (launchd monitors this script)
while "$TMUX" has-session -t "$SESSION" 2>/dev/null; do
    sleep 10
done

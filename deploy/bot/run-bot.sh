#!/bin/bash
SESSION="claude-bot"
TMUX="/opt/homebrew/bin/tmux"
BOT="$(cd "$(dirname "$0")/../.." && pwd)/claude-bot"

# 기존 세션이 있으면 내부 프로세스까지 확실히 정리
if "$TMUX" has-session -t "$SESSION" 2>/dev/null; then
    for pid in $("$TMUX" list-panes -t "$SESSION" -F '#{pane_pid}' 2>/dev/null); do
        pkill -TERM -P "$pid" 2>/dev/null
        kill -TERM "$pid" 2>/dev/null
    done
    sleep 1
    "$TMUX" kill-session -t "$SESSION" 2>/dev/null
fi

# 새 tmux 세션에서 bot 실행
"$TMUX" new-session -d -s "$SESSION" \
    "/bin/zsh -c '
export HOME=$HOME
export TERMINAL_EMULATOR=launchd
unset TMUX
source \$HOME/.zshrc 2>/dev/null
for p in \$(find \$HOME/.rc/ -not -type d | sort); do source \$p 2>/dev/null; done
exec $BOT
'"

# 세션이 살아있는 동안 대기 (launchd가 이 스크립트를 감시)
while "$TMUX" has-session -t "$SESSION" 2>/dev/null; do
    sleep 10
done

#!/bin/bash
SESSION="claude-channel"
TMUX="/opt/homebrew/bin/tmux"
CLAUDE="/opt/homebrew/bin/claude"

# 설정 파일 로드 (있으면)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG="$SCRIPT_DIR/channel.env"
if [[ -f "$CONFIG" ]]; then
    source "$CONFIG"
fi

# 환경 변수로 커스텀 가능한 옵션
CLAUDE_CHANNELS="${CLAUDE_CHANNELS:-plugin:discord@claude-plugins-official}"
CLAUDE_EXTRA_ARGS="${CLAUDE_EXTRA_ARGS:-}"

# 기존 세션이 있으면 내부 프로세스까지 확실히 정리
if "$TMUX" has-session -t "$SESSION" 2>/dev/null; then
    for pid in $("$TMUX" list-panes -t "$SESSION" -F '#{pane_pid}' 2>/dev/null); do
        pkill -TERM -P "$pid" 2>/dev/null
        kill -TERM "$pid" 2>/dev/null
    done
    sleep 1
    "$TMUX" kill-session -t "$SESSION" 2>/dev/null
fi

# 새 tmux 세션에서 claude 실행
# .zshrc source 후 else 분기의 PATH=$ORIGIN_PATH 리셋을 보정하기 위해 ~/.rc/ 재로드
"$TMUX" new-session -d -s "$SESSION" \
    "/bin/zsh -c '
export HOME=$HOME
export TERMINAL_EMULATOR=launchd
export CLAUDE_DISCORD_SESSION=1
unset TMUX
source \$HOME/.zshrc 2>/dev/null
for p in \$(find \$HOME/.rc/ -not -type d | sort); do source \$p 2>/dev/null; done
echo \"PATH=\$PATH\"
echo \"bun: \$(which bun 2>&1)\"
echo \"---\"
exec $CLAUDE --debug --channels $CLAUDE_CHANNELS --dangerously-skip-permissions $CLAUDE_EXTRA_ARGS
'"

# 세션이 살아있는 동안 대기 (launchd가 이 스크립트를 감시)
while "$TMUX" has-session -t "$SESSION" 2>/dev/null; do
    sleep 10
done

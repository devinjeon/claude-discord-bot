#!/usr/bin/env bash
set -euo pipefail

LABEL_BOT="com.devin.claude-bot"
LABEL_CHANNEL="com.devin.claude-channel"
TMUX="/opt/homebrew/bin/tmux"
HOOK_DST="$HOME/.claude/hooks/discord-restart-notify.sh"
SETTINGS="$HOME/.claude/settings.json"

echo "=== claude-discord-bot uninstall ==="

# 1. LaunchAgent 중지 및 제거
for label in "$LABEL_BOT" "$LABEL_CHANNEL"; do
  if launchctl list "$label" &>/dev/null; then
    launchctl bootout "gui/$(id -u)/$label" 2>/dev/null || true
    echo "[launchd] Stopped: $label"
  fi
  plist="$HOME/Library/LaunchAgents/$label.plist"
  if [[ -L "$plist" || -f "$plist" ]]; then
    rm "$plist"
    echo "[launchd] Removed: $plist"
  fi
done

# 2. tmux 세션 정리
for session in "claude-bot" "claude-channel"; do
  if "$TMUX" has-session -t "$session" 2>/dev/null; then
    for pid in $("$TMUX" list-panes -t "$session" -F '#{pane_pid}' 2>/dev/null); do
      pkill -TERM -P "$pid" 2>/dev/null
      kill -TERM "$pid" 2>/dev/null
    done
    sleep 1
    "$TMUX" kill-session -t "$session" 2>/dev/null
    echo "[tmux] Killed session: $session"
  fi
done

# 3. Hook 제거
if [[ -L "$HOOK_DST" ]]; then
  rm "$HOOK_DST"
  echo "[hook] Removed: $HOOK_DST"
fi

if [[ -f "$SETTINGS" ]]; then
  HOOK_CMD="$HOOK_DST"
  jq --arg cmd "$HOOK_CMD" '
    if .hooks.SessionStart then
      .hooks.SessionStart = [.hooks.SessionStart[] | select(.hooks | all(.command != $cmd))]
      | if .hooks.SessionStart == [] then del(.hooks.SessionStart) else . end
      | if .hooks == {} then del(.hooks) else . end
    else . end
  ' "$SETTINGS" > "$SETTINGS.tmp" && mv "$SETTINGS.tmp" "$SETTINGS"
  echo "[hook] Removed SessionStart hook from settings.json"
fi

echo "=== Uninstall done ==="

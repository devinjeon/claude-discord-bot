#!/usr/bin/env bash
set -euo pipefail

LABEL_BOT="com.devin.claude-bot"
LABEL_CHANNEL="com.devin.claude-channel"
TMUX="$(command -v tmux 2>/dev/null || echo tmux)"
HOOK_DST="$HOME/.claude/hooks/discord-restart-notify.sh"
SETTINGS="$HOME/.claude/settings.json"

echo "=== claude-discord-bot uninstall ==="

# 1. Stop and remove LaunchAgents
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

# 2. Kill tmux sessions
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

# 3. Remove hook
if [[ -L "$HOOK_DST" ]]; then
  rm "$HOOK_DST"
  echo "[hook] Removed: $HOOK_DST"
fi

if [[ -f "$SETTINGS" ]] && jq empty "$SETTINGS" 2>/dev/null; then
  HOOK_CMD="$HOOK_DST"
  UPDATED=$(jq --arg cmd "$HOOK_CMD" '
    if .hooks.SessionStart then
      .hooks.SessionStart = [.hooks.SessionStart[] | select(.hooks | all(.command != $cmd))]
      | if .hooks.SessionStart == [] then del(.hooks.SessionStart) else . end
      | if .hooks == {} then del(.hooks) else . end
    else . end
  ' "$SETTINGS")
  if [[ -n "$UPDATED" ]]; then
    echo "$UPDATED" > "$SETTINGS"
    echo "[hook] Removed SessionStart hook from settings.json"
  fi
fi

echo "=== Uninstall done ==="

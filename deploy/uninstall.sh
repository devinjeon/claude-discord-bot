#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LABEL_BOT="com.devin.claude-bot"
INSTANCES_CONF="$SCRIPT_DIR/instances.conf"
TMUX="$(command -v tmux 2>/dev/null || echo tmux)"
HOOK_DST="$HOME/.claude/hooks/discord-restart-notify.sh"
SETTINGS="$HOME/.claude/settings.json"

echo "=== claude-discord-bot uninstall ==="

# 1. Stop and remove bot LaunchAgent
if launchctl list "$LABEL_BOT" &>/dev/null; then
  launchctl bootout "gui/$(id -u)/$LABEL_BOT" 2>/dev/null || true
  echo "[launchd] Stopped: $LABEL_BOT"
fi
plist="$HOME/Library/LaunchAgents/$LABEL_BOT.plist"
if [[ -L "$plist" || -f "$plist" ]]; then
  rm "$plist"
  echo "[launchd] Removed: $plist"
fi

# 2. Stop and remove instance LaunchAgents + kill tmux sessions
if [[ -f "$INSTANCES_CONF" ]]; then
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="$(echo "$line" | sed 's/#.*//' | xargs)"
    [[ -z "$line" ]] && continue
    instance_name="$(echo "$line" | awk '{print $1}')"

    label="com.devin.claude-channel-${instance_name}"
    session="claude-channel-${instance_name}"

    # Stop LaunchAgent
    if launchctl list "$label" &>/dev/null; then
      launchctl bootout "gui/$(id -u)/$label" 2>/dev/null || true
      echo "[launchd] Stopped: $label"
    fi
    plist="$HOME/Library/LaunchAgents/$label.plist"
    if [[ -L "$plist" || -f "$plist" ]]; then
      rm "$plist"
      echo "[launchd] Removed: $plist"
    fi

    # Kill tmux session
    if "$TMUX" has-session -t "$session" 2>/dev/null; then
      for pid in $("$TMUX" list-panes -t "$session" -F '#{pane_pid}' 2>/dev/null); do
        pkill -TERM -P "$pid" 2>/dev/null
        kill -TERM "$pid" 2>/dev/null
      done
      sleep 1
      "$TMUX" kill-session -t "$session" 2>/dev/null
      echo "[tmux] Killed session: $session"
    fi
  done < "$INSTANCES_CONF"
fi

# 3. Also clean up legacy single-instance if present
for label in "com.devin.claude-channel"; do
  if launchctl list "$label" &>/dev/null; then
    launchctl bootout "gui/$(id -u)/$label" 2>/dev/null || true
    echo "[launchd] Stopped: $label (legacy)"
  fi
  plist="$HOME/Library/LaunchAgents/$label.plist"
  if [[ -L "$plist" || -f "$plist" ]]; then
    rm "$plist"
    echo "[launchd] Removed: $plist (legacy)"
  fi
done

# 4. Kill bot tmux session
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

# 5. Remove hook
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

# 6. Remove instances.json
INSTANCES_JSON="$HOME/.claude/channels/discord/instances.json"
if [[ -f "$INSTANCES_JSON" ]]; then
  rm "$INSTANCES_JSON"
  echo "[config] Removed: $INSTANCES_JSON"
fi

echo "=== Uninstall done ==="

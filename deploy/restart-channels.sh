#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTANCES_CONF="$SCRIPT_DIR/instances.conf"
LABEL_BOT="com.devin.claude-bot"

RESTART_BOT=false
ONLY=""
for arg in "$@"; do
  case "$arg" in
    --with-bot) RESTART_BOT=true ;;
    --only=*)   ONLY="${arg#--only=}" ;;
  esac
done

# Restart a LaunchAgent: bootout then bootstrap. The run-channel.sh entrypoint
# kills the old tmux session and starts a fresh one on relaunch.
restart_service() {
  local label="$1" plist="$2"
  if [[ ! -f "$plist" ]]; then
    echo "[skip] Plist not found: $plist"
    return
  fi
  if launchctl list "$label" &>/dev/null; then
    launchctl bootout "gui/$(id -u)/$label" 2>/dev/null || true
  fi
  launchctl bootstrap "gui/$(id -u)" "$plist" 2>/dev/null || true
  echo "[launchd] Restarted: $label"
}

echo "=== Restarting channel instances ==="

if [[ -f "$INSTANCES_CONF" ]]; then
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="$(echo "$line" | sed 's/#.*//' | xargs)"
    [[ -z "$line" ]] && continue
    instance_name="$(echo "$line" | awk '{print $1}')"
    [[ -n "$ONLY" && "$ONLY" != "$instance_name" ]] && continue
    label="com.devin.claude-channel-${instance_name}"
    plist="$HOME/Library/LaunchAgents/$label.plist"
    restart_service "$label" "$plist"
  done < "$INSTANCES_CONF"
else
  label="com.devin.claude-channel"
  plist="$HOME/Library/LaunchAgents/$label.plist"
  restart_service "$label" "$plist"
fi

if [[ "$RESTART_BOT" == "true" ]]; then
  restart_service "$LABEL_BOT" "$HOME/Library/LaunchAgents/$LABEL_BOT.plist"
fi

echo "=== Done ==="

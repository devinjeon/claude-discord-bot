#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTANCES_CONF="$SCRIPT_DIR/instances.conf"
TMUX="$(command -v tmux 2>/dev/null || echo tmux)"
DISCORD_CONFIG_DIR="$HOME/.claude/channels/discord"

usage() {
  cat <<EOF
Usage:
  $(basename "$0") add    NAME CHANNEL_ID WORKING_DIR
  $(basename "$0") remove NAME

Examples:
  $(basename "$0") add my-project 123456789012345678 ~/my-project
  $(basename "$0") remove my-project
EOF
  exit 1
}

[[ $# -lt 2 ]] && usage

ACTION="$1"
NAME="$2"

case "$ACTION" in
  add)
    [[ $# -lt 4 ]] && usage
    CHANNEL_ID="$3"
    WORKING_DIR="$4"

    # Validate name doesn't already exist
    if grep -q "^${NAME}[[:space:]]" "$INSTANCES_CONF" 2>/dev/null; then
      echo "[error] Instance '${NAME}' already exists in instances.conf"
      exit 1
    fi

    # Append to instances.conf
    printf '%-40s %s  %s\n' "$NAME" "$CHANNEL_ID" "$WORKING_DIR" >> "$INSTANCES_CONF"
    echo "[config] Added '${NAME}' to instances.conf"

    # Run install without build
    bash "$SCRIPT_DIR/install.sh" --no-build
    ;;

  remove)
    # Validate name exists
    if ! grep -q "^${NAME}[[:space:]]" "$INSTANCES_CONF" 2>/dev/null; then
      echo "[error] Instance '${NAME}' not found in instances.conf"
      exit 1
    fi

    label="com.devin.claude-channel-${NAME}"
    session="claude-channel-${NAME}"
    plist="$HOME/Library/LaunchAgents/$label.plist"
    state_dir="$HOME/.claude/channels/discord-${NAME}"

    # 1. Stop LaunchAgent
    if launchctl list "$label" &>/dev/null; then
      launchctl bootout "gui/$(id -u)/$label" 2>/dev/null || true
      echo "[launchd] Stopped: $label"
    fi

    # 2. Remove plist
    if [[ -f "$plist" ]]; then
      rm "$plist"
      echo "[launchd] Removed: $plist"
    fi

    # 3. Kill tmux session
    if "$TMUX" has-session -t "$session" 2>/dev/null; then
      for pid in $("$TMUX" list-panes -t "$session" -F '#{pane_pid}' 2>/dev/null); do
        pkill -TERM -P "$pid" 2>/dev/null || true
        kill -TERM "$pid" 2>/dev/null || true
      done
      sleep 1
      "$TMUX" kill-session -t "$session" 2>/dev/null || true
      echo "[tmux] Killed session: $session"
    fi

    # 4. Remove per-instance state dir
    if [[ -d "$state_dir" ]]; then
      rm -rf "$state_dir"
      echo "[config] Removed state dir: $state_dir"
    fi

    # 5. Remove from instances.conf
    sed -i '' "/^${NAME}[[:space:]]/d" "$INSTANCES_CONF"
    echo "[config] Removed '${NAME}' from instances.conf"

    # 6. Rebuild instances.json and restart bot (to drop the removed channel)
    bash "$SCRIPT_DIR/install.sh" --no-build

    echo "=== Channel '${NAME}' removed ==="
    ;;

  *)
    usage
    ;;
esac

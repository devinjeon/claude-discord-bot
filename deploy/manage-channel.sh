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

    # 6. Rebuild instances.json and signal bot to reload (no restart)
    INSTANCES_JSON='{"instances":['
    FIRST=true
    while IFS= read -r line || [[ -n "$line" ]]; do
      line="$(echo "$line" | sed 's/#.*//' | xargs)"
      [[ -z "$line" ]] && continue
      inst_name="$(echo "$line" | awk '{print $1}')"
      ch_id="$(echo "$line" | awk '{print $2}')"
      tmux_sess="claude-channel-${inst_name}"
      if [[ "$FIRST" == "true" ]]; then
        FIRST=false
      else
        INSTANCES_JSON+=','
      fi
      INSTANCES_JSON+="{\"name\":\"${inst_name}\",\"channel_id\":\"${ch_id}\",\"tmux_session\":\"${tmux_sess}\"}"
    done < "$INSTANCES_CONF"
    INSTANCES_JSON+=']}'

    INSTANCES_JSON_FILE="$DISCORD_CONFIG_DIR/instances.json"
    mkdir -p "$DISCORD_CONFIG_DIR"
    echo "$INSTANCES_JSON" | python3 -m json.tool > "$INSTANCES_JSON_FILE" 2>/dev/null \
      || echo "$INSTANCES_JSON" > "$INSTANCES_JSON_FILE"
    echo "[config] Updated instances.json"

    # Send SIGHUP to bot for hot-reload (no restart needed)
    BOT_LABEL="com.devin.claude-bot"
    BOT_PID=$(launchctl list "$BOT_LABEL" 2>/dev/null | awk 'NR==1{print $1}')
    # launchctl list <label> outputs: PID Status Label — first field is PID (or "-" if not running)
    if [[ "$BOT_PID" == "-" || -z "$BOT_PID" ]]; then
      BOT_PID=$(pgrep -x "claude-bot" 2>/dev/null | head -1)
    fi
    if [[ -n "$BOT_PID" ]]; then
      kill -HUP "$BOT_PID" 2>/dev/null && echo "[bot] Sent SIGHUP to bot (PID $BOT_PID) for hot-reload" \
        || echo "[bot] Failed to send SIGHUP to bot"
    else
      echo "[bot] Bot process not found — will pick up changes on next start"
    fi

    echo "=== Channel '${NAME}' removed ==="
    ;;

  *)
    usage
    ;;
esac

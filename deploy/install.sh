#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

LABEL_BOT="com.devin.claude-bot"
INSTANCES_CONF="$SCRIPT_DIR/instances.conf"
HOOK_SRC="$SCRIPT_DIR/claude-channel/discord-restart-notify.sh"
HOOK_DST="$HOME/.claude/hooks/discord-restart-notify.sh"
SETTINGS="$HOME/.claude/settings.json"
DISCORD_CONFIG_DIR="$HOME/.claude/channels/discord"

FORCE_RESTART=false
if [[ "${1:-}" == "--force" ]]; then
  FORCE_RESTART=true
fi

echo "=== claude-discord-bot install ==="

# Tracking arrays for selective restart
RESTART_BOT=false
RESTART_ALL_CHANNELS=false
declare -a RESTART_CHANNELS=()

# Helper: md5 checksum (portable macOS/Linux)
file_md5() {
  if [[ -f "$1" ]]; then
    md5 -q "$1" 2>/dev/null || md5sum "$1" | awk '{print $1}'
  else
    echo ""
  fi
}

# Helper: restart a LaunchAgent only if needed
restart_service() {
  local label="$1" plist="$2"
  if launchctl list "$label" &>/dev/null; then
    launchctl bootout "gui/$(id -u)/$label" 2>/dev/null || true
  fi
  launchctl bootstrap "gui/$(id -u)" "$plist" 2>/dev/null || true
  echo "[launchd] Restarted: $label"
}

# 0. Capture current shell environment
USER_HOME="$HOME"
USER_SHELL="$(basename "$SHELL")"
USER_PATH="$PATH"

TMUX_PATH="$(command -v tmux 2>/dev/null || true)"
if [[ -z "$TMUX_PATH" ]]; then
  echo "[error] tmux not found in PATH. Install tmux first." >&2
  exit 1
fi

CLAUDE_PATH="$(command -v claude 2>/dev/null || true)"
if [[ -z "$CLAUDE_PATH" ]]; then
  echo "[error] claude not found in PATH. Install Claude Code first." >&2
  exit 1
fi

echo "[env] HOME=$USER_HOME"
echo "[env] SHELL=$USER_SHELL"
echo "[env] tmux=$TMUX_PATH"
echo "[env] claude=$CLAUDE_PATH"

# 1. Build
echo "[build] Building claude-bot..."
cd "$PROJECT_DIR"
if ! command -v go &>/dev/null; then
  echo "[error] go not found in PATH. Install Go first." >&2
  exit 1
fi
BINARY_MD5_BEFORE="$(file_md5 "$PROJECT_DIR/claude-bot")"
go build -o claude-bot ./cmd/claude-bot
BINARY_MD5_AFTER="$(file_md5 "$PROJECT_DIR/claude-bot")"
if [[ "$BINARY_MD5_BEFORE" != "$BINARY_MD5_AFTER" ]]; then
  echo "[build] Binary changed"
  RESTART_BOT=true
else
  echo "[build] Binary unchanged"
fi

# 3. Set script permissions
chmod +x "$SCRIPT_DIR/bot/run-bot.sh"
chmod +x "$SCRIPT_DIR/claude-channel/run-channel.sh"
chmod +x "$SCRIPT_DIR/claude-channel/discord-restart-notify.sh"

# 4. Save environment snapshot (sourced by run-*.sh)
ENV_SNAPSHOT="$SCRIPT_DIR/env.generated.sh"
cat > "$ENV_SNAPSHOT" <<EOF
# Captures the installing user's shell environment.
INSTALL_HOME="$USER_HOME"
INSTALL_SHELL="$USER_SHELL"
INSTALL_PATH="$USER_PATH"
INSTALL_TMUX="$TMUX_PATH"
INSTALL_CLAUDE="$CLAUDE_PATH"
EOF
echo "[env] Saved environment snapshot: $ENV_SNAPSHOT"

# 4b. Check if channel config files changed
STATE_FILE="$SCRIPT_DIR/.install-state"
CHANNEL_ENV_FILE="$SCRIPT_DIR/claude-channel/channel.env"
PRE_RUN_FILE="$SCRIPT_DIR/claude-channel/pre-run.sh"
CHANNEL_ENV_MD5="$(file_md5 "$CHANNEL_ENV_FILE")"
PRE_RUN_MD5="$(file_md5 "$PRE_RUN_FILE")"
ENV_SNAPSHOT_MD5="$(file_md5 "$ENV_SNAPSHOT")"

if [[ -f "$STATE_FILE" ]]; then
  PREV_CHANNEL_ENV_MD5="$(grep '^CHANNEL_ENV=' "$STATE_FILE" 2>/dev/null | cut -d= -f2- || true)"
  PREV_PRE_RUN_MD5="$(grep '^PRE_RUN=' "$STATE_FILE" 2>/dev/null | cut -d= -f2- || true)"
  PREV_ENV_SNAPSHOT_MD5="$(grep '^ENV_SNAPSHOT=' "$STATE_FILE" 2>/dev/null | cut -d= -f2- || true)"
  if [[ "$CHANNEL_ENV_MD5" != "$PREV_CHANNEL_ENV_MD5" ]]; then
    echo "[config] channel.env changed"
    RESTART_ALL_CHANNELS=true
  fi
  if [[ "$PRE_RUN_MD5" != "$PREV_PRE_RUN_MD5" ]]; then
    echo "[config] pre-run.sh changed"
    RESTART_ALL_CHANNELS=true
  fi
  if [[ "$ENV_SNAPSHOT_MD5" != "$PREV_ENV_SNAPSHOT_MD5" ]]; then
    echo "[config] env.generated.sh changed"
    RESTART_ALL_CHANNELS=true
    RESTART_BOT=true
  fi
else
  echo "[config] No previous state found, will restart all"
  RESTART_ALL_CHANNELS=true
  RESTART_BOT=true
fi

# 5. Generate LaunchAgent plist files
mkdir -p "$USER_HOME/Library/LaunchAgents"

generate_plist() {
  local label="$1" script="$2" log_name="$3" dest="$4"
  shift 4
  # Remaining args are key-value pairs for EnvironmentVariables
  local env_vars=""
  while [[ $# -ge 2 ]]; do
    env_vars+="        <key>$1</key>
        <string>$2</string>
"
    shift 2
  done

  cat > "$dest" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${label}</string>
    <key>ProgramArguments</key>
    <array>
        <string>/bin/bash</string>
        <string>${script}</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${USER_HOME}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${USER_HOME}/Library/Logs/${log_name}.stdout.log</string>
    <key>StandardErrorPath</key>
    <string>${USER_HOME}/Library/Logs/${log_name}.stderr.log</string>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>${USER_PATH}</string>
        <key>HOME</key>
        <string>${USER_HOME}</string>
${env_vars}    </dict>
</dict>
</plist>
PLIST
}

# 6. Parse instances.conf and generate per-instance LaunchAgents + instances.json
INSTANCES_JSON='{"instances":['
FIRST_INSTANCE=true

if [[ -f "$INSTANCES_CONF" ]]; then
  MULTI_INSTANCE=true
  echo "[instances] Reading $INSTANCES_CONF"

  while IFS= read -r line || [[ -n "$line" ]]; do
    # Skip comments and empty lines
    line="$(echo "$line" | sed 's/#.*//' | xargs)"
    [[ -z "$line" ]] && continue

    # Parse: instance_name channel_id working_directory
    instance_name="$(echo "$line" | awk '{print $1}')"
    channel_id="$(echo "$line" | awk '{print $2}')"
    working_dir="$(echo "$line" | awk '{print $3}')"

    # Expand ~
    working_dir="${working_dir/#\~/$USER_HOME}"

    label_channel="com.devin.claude-channel-${instance_name}"
    tmux_session="claude-channel-${instance_name}"

    # Generate channel LaunchAgent to temp, compare with existing
    channel_plist="$USER_HOME/Library/LaunchAgents/$label_channel.plist"
    channel_plist_tmp="${channel_plist}.tmp"
    generate_plist "$label_channel" \
      "$SCRIPT_DIR/claude-channel/run-channel.sh" \
      "claude-channel-${instance_name}" \
      "$channel_plist_tmp" \
      "INSTANCE_NAME" "$instance_name" \
      "WORKING_DIR" "$working_dir" \
      "DISCORD_CHANNEL_ID" "$channel_id"

    if ! diff -q "$channel_plist_tmp" "$channel_plist" &>/dev/null; then
      echo "[launchd] Plist changed: $label_channel (${instance_name} -> ${working_dir})"
      mv "$channel_plist_tmp" "$channel_plist"
      RESTART_CHANNELS+=("$instance_name")
    else
      echo "[launchd] Plist unchanged: $label_channel"
      rm -f "$channel_plist_tmp"
    fi

    # Create per-instance Discord state dir with its own access.json and .env
    state_dir="$HOME/.claude/channels/discord-${instance_name}"
    mkdir -p "$state_dir"

    # Copy bot token from main .env
    main_env="$DISCORD_CONFIG_DIR/.env"
    if [[ -f "$main_env" ]]; then
      bot_token="$(grep '^DISCORD_BOT_TOKEN=' "$main_env" | cut -d= -f2-)"
      cat > "$state_dir/.env" <<ENVFILE
DISCORD_BOT_TOKEN=${bot_token}
ENVFILE
    fi

    # Write per-instance access.json with only this channel
    main_access="$DISCORD_CONFIG_DIR/access.json"
    if [[ -f "$main_access" ]]; then
      allow_from="$(jq -c '.allowFrom // []' "$main_access")"
      dm_policy="$(jq -r '.dmPolicy // "pairing"' "$main_access")"
    else
      allow_from='[]'
      dm_policy="pairing"
    fi
    cat > "$state_dir/access.json" <<ACCESSJSON
{
  "dmPolicy": "${dm_policy}",
  "allowFrom": ${allow_from},
  "groups": {
    "${channel_id}": { "requireMention": false, "allowFrom": [] }
  },
  "pending": {}
}
ACCESSJSON
    echo "[config] Written: $state_dir/access.json (channel ${channel_id})"

    # Add to instances.json
    if [[ "$FIRST_INSTANCE" == "true" ]]; then
      FIRST_INSTANCE=false
    else
      INSTANCES_JSON+=','
    fi
    INSTANCES_JSON+="{\"name\":\"${instance_name}\",\"channel_id\":\"${channel_id}\",\"tmux_session\":\"${tmux_session}\"}"

  done < "$INSTANCES_CONF"
else
  # No instances.conf: single-instance mode using standard Claude Channel config.
  # Uses ~/.claude/channels/discord/ as-is (the default DISCORD_STATE_DIR).
  # The bot reads channel ID from access.json or .env automatically.
  MULTI_INSTANCE=false
  LABEL_CHANNEL="com.devin.claude-channel"

  echo "[instances] No instances.conf found, using single-instance mode"

  # Generate single channel LaunchAgent to temp, compare with existing
  channel_plist="$USER_HOME/Library/LaunchAgents/$LABEL_CHANNEL.plist"
  channel_plist_tmp="${channel_plist}.tmp"
  generate_plist "$LABEL_CHANNEL" \
    "$SCRIPT_DIR/claude-channel/run-channel.sh" \
    "claude-channel" \
    "$channel_plist_tmp"

  if ! diff -q "$channel_plist_tmp" "$channel_plist" &>/dev/null; then
    echo "[launchd] Plist changed: $LABEL_CHANNEL (default -> ~/)"
    mv "$channel_plist_tmp" "$channel_plist"
    RESTART_ALL_CHANNELS=true
  else
    echo "[launchd] Plist unchanged: $LABEL_CHANNEL"
    rm -f "$channel_plist_tmp"
  fi
fi

INSTANCES_JSON+=']}'

# Write instances.json for the bot to read (empty in single-instance mode,
# bot falls back to reading channel ID from access.json/.env)
mkdir -p "$DISCORD_CONFIG_DIR"
echo "$INSTANCES_JSON" | python3 -m json.tool > "$DISCORD_CONFIG_DIR/instances.json" 2>/dev/null \
  || echo "$INSTANCES_JSON" > "$DISCORD_CONFIG_DIR/instances.json"
echo "[config] Written: $DISCORD_CONFIG_DIR/instances.json"

# 7. Generate bot LaunchAgent
#    Read optional bot.env for extra environment variables
BOT_ENV_ARGS=()
BOT_ENV_FILE="$SCRIPT_DIR/bot/bot.env"
if [[ -f "$BOT_ENV_FILE" ]]; then
  while IFS='=' read -r key value || [[ -n "$key" ]]; do
    key="$(echo "$key" | xargs)"
    [[ -z "$key" || "$key" == \#* ]] && continue
    value="$(echo "$value" | xargs)"
    BOT_ENV_ARGS+=("$key" "$value")
    echo "[bot.env] $key=$value"
  done < "$BOT_ENV_FILE"
fi

BOT_PLIST="$USER_HOME/Library/LaunchAgents/$LABEL_BOT.plist"
BOT_PLIST_TMP="${BOT_PLIST}.tmp"
generate_plist "$LABEL_BOT" "$SCRIPT_DIR/bot/run-bot.sh" "claude-bot" \
  "$BOT_PLIST_TMP" \
  "${BOT_ENV_ARGS[@]+"${BOT_ENV_ARGS[@]}"}"

if ! diff -q "$BOT_PLIST_TMP" "$BOT_PLIST" &>/dev/null; then
  echo "[launchd] Bot plist changed"
  mv "$BOT_PLIST_TMP" "$BOT_PLIST"
  RESTART_BOT=true
else
  echo "[launchd] Bot plist unchanged"
  rm -f "$BOT_PLIST_TMP"
fi

# 8. Install Claude Code SessionStart hook
mkdir -p "$(dirname "$HOOK_DST")"
ln -sf "$HOOK_SRC" "$HOOK_DST"
echo "[hook] Linked: $HOOK_DST"

if [[ -f "$SETTINGS" ]]; then
  HOOK_CMD="$HOOK_DST"
  HAS_HOOK=$(jq --arg cmd "$HOOK_CMD" '
    .hooks.SessionStart // [] | map(.hooks[]? | select(.command == $cmd)) | length
  ' "$SETTINGS")

  if [[ "$HAS_HOOK" == "0" ]]; then
    jq --arg cmd "$HOOK_CMD" '
      .hooks.SessionStart = (.hooks.SessionStart // []) + [{
        "matcher": "",
        "hooks": [{"type": "command", "command": $cmd}]
      }]
    ' "$SETTINGS" > "$SETTINGS.tmp" && mv "$SETTINGS.tmp" "$SETTINGS"
    echo "[hook] Added SessionStart hook to settings.json"
  else
    echo "[hook] SessionStart hook already exists, skipping"
  fi
fi

# 9. Selective restart — only restart services that changed
if [[ "$FORCE_RESTART" == "true" ]]; then
  echo "[restart] --force: restarting all services"
  RESTART_BOT=true
  RESTART_ALL_CHANNELS=true
fi

# Restart channel instances
if [[ "$MULTI_INSTANCE" == "true" ]]; then
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="$(echo "$line" | sed 's/#.*//' | xargs)"
    [[ -z "$line" ]] && continue
    instance_name="$(echo "$line" | awk '{print $1}')"
    label="com.devin.claude-channel-${instance_name}"
    plist="$USER_HOME/Library/LaunchAgents/$label.plist"

    should_restart=false
    if [[ "$RESTART_ALL_CHANNELS" == "true" ]]; then
      should_restart=true
    else
      for name in "${RESTART_CHANNELS[@]+"${RESTART_CHANNELS[@]}"}"; do
        if [[ "$name" == "$instance_name" ]]; then
          should_restart=true
          break
        fi
      done
    fi

    if [[ "$should_restart" == "true" ]]; then
      restart_service "$label" "$plist"
    else
      # Ensure service is running even if no restart needed
      if ! launchctl list "$label" &>/dev/null; then
        restart_service "$label" "$plist"
      else
        echo "[launchd] No change: $label"
      fi
    fi
  done < "$INSTANCES_CONF"
else
  label="com.devin.claude-channel"
  plist="$USER_HOME/Library/LaunchAgents/$label.plist"
  if [[ "$RESTART_ALL_CHANNELS" == "true" ]]; then
    restart_service "$label" "$plist"
  elif ! launchctl list "$label" &>/dev/null; then
    restart_service "$label" "$plist"
  else
    echo "[launchd] No change: $label"
  fi
fi

# Restart bot
BOT_PLIST_PATH="$USER_HOME/Library/LaunchAgents/$LABEL_BOT.plist"
if [[ "$RESTART_BOT" == "true" ]]; then
  restart_service "$LABEL_BOT" "$BOT_PLIST_PATH"
elif ! launchctl list "$LABEL_BOT" &>/dev/null; then
  restart_service "$LABEL_BOT" "$BOT_PLIST_PATH"
else
  echo "[launchd] No change: $LABEL_BOT"
fi

# 10. Save state for next run
cat > "$STATE_FILE" <<STATEFILE
CHANNEL_ENV=$CHANNEL_ENV_MD5
PRE_RUN=$PRE_RUN_MD5
ENV_SNAPSHOT=$(file_md5 "$ENV_SNAPSHOT")
STATEFILE

echo "=== Install done ==="

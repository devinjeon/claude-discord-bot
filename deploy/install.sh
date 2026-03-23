#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

LABEL_BOT="com.devin.claude-bot"
LABEL_CHANNEL="com.devin.claude-channel"
HOOK_SRC="$SCRIPT_DIR/claude-channel/discord-restart-notify.sh"
HOOK_DST="$HOME/.claude/hooks/discord-restart-notify.sh"
SETTINGS="$HOME/.claude/settings.json"

echo "=== claude-discord-bot install ==="

# 1. 빌드
echo "[build] Building claude-bot..."
cd "$PROJECT_DIR"
if ! command -v go &>/dev/null; then
  echo "[error] go not found in PATH. Install Go first." >&2
  exit 1
fi
go build -o claude-bot ./cmd/claude-bot
echo "[build] Done"

# 2. 기존 서비스 중지
for label in "$LABEL_BOT" "$LABEL_CHANNEL"; do
  if launchctl list "$label" &>/dev/null; then
    launchctl bootout "gui/$(id -u)/$label" 2>/dev/null || true
    echo "[launchd] Stopped: $label"
  fi
done

# 3. 스크립트 실행 권한
chmod +x "$SCRIPT_DIR/bot/run-bot.sh"
chmod +x "$SCRIPT_DIR/claude-channel/run-channel.sh"
chmod +x "$SCRIPT_DIR/claude-channel/discord-restart-notify.sh"

# 4. LaunchAgent plist 생성 (경로를 동적으로 주입)
mkdir -p "$HOME/Library/LaunchAgents"

generate_plist() {
  local label="$1" script="$2" log_name="$3" dest="$4"
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
    <string>${HOME}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${HOME}/Library/Logs/${log_name}.stdout.log</string>
    <key>StandardErrorPath</key>
    <string>${HOME}/Library/Logs/${log_name}.stderr.log</string>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
        <key>HOME</key>
        <string>${HOME}</string>
    </dict>
</dict>
</plist>
PLIST
}

generate_plist "$LABEL_BOT" "$SCRIPT_DIR/bot/run-bot.sh" "claude-bot" \
  "$HOME/Library/LaunchAgents/$LABEL_BOT.plist"
generate_plist "$LABEL_CHANNEL" "$SCRIPT_DIR/claude-channel/run-channel.sh" "claude-channel" \
  "$HOME/Library/LaunchAgents/$LABEL_CHANNEL.plist"
echo "[launchd] Generated plist files"

# 5. Claude Code SessionStart hook 설치
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

# 6. LaunchAgent 로드
for label in "$LABEL_CHANNEL" "$LABEL_BOT"; do
  plist="$HOME/Library/LaunchAgents/$label.plist"
  if launchctl list "$label" &>/dev/null; then
    launchctl bootout "gui/$(id -u)/$label" 2>/dev/null || true
  fi
  launchctl bootstrap "gui/$(id -u)" "$plist"
  echo "[launchd] Loaded: $label"
done

echo "=== Install done ==="

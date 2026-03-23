#!/bin/bash
# Discord 재시작 알림 - CLAUDE_DISCORD_SESSION 환경변수가 있을 때만 실행

[ "$CLAUDE_DISCORD_SESSION" != "1" ] && exit 0

ENV_FILE="$HOME/.claude/channels/discord/.env"
CHANNEL_ID="1485304276107395164"

[ ! -f "$ENV_FILE" ] && exit 0

TOKEN=$(grep '^DISCORD_BOT_TOKEN=' "$ENV_FILE" | cut -d'=' -f2-)
[ -z "$TOKEN" ] && exit 0

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "https://discord.com/api/v10/channels/${CHANNEL_ID}/messages" \
  -H "Authorization: Bot ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"content\":\"[$(date '+%Y-%m-%d %H:%M:%S')] Claude가 재시작되었습니다.\"}")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
if [ "$HTTP_CODE" -lt 200 ] || [ "$HTTP_CODE" -ge 300 ]; then
  echo "[discord-restart-notify] Failed to send notification (HTTP $HTTP_CODE)" >&2
fi

exit 0

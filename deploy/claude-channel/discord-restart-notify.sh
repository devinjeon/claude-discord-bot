#!/bin/bash
# Discord restart notification — only runs when CLAUDE_DISCORD_SESSION is set

[ "$CLAUDE_DISCORD_SESSION" != "1" ] && exit 0

CONFIG_DIR="${DISCORD_STATE_DIR:-$HOME/.claude/channels/discord}"
ENV_FILE="$CONFIG_DIR/.env"
ACCESS_FILE="$CONFIG_DIR/access.json"

[ ! -f "$ENV_FILE" ] && exit 0

TOKEN=$(grep '^DISCORD_BOT_TOKEN=' "$ENV_FILE" | cut -d'=' -f2-)
[ -z "$TOKEN" ] && exit 0

# Channel ID lookup: .env first, then access.json groups
CHANNEL_ID=$(grep '^DISCORD_CHANNEL_ID=' "$ENV_FILE" | cut -d'=' -f2-)
if [ -z "$CHANNEL_ID" ] && [ -f "$ACCESS_FILE" ]; then
  CHANNEL_ID=$(python3 -c "import json,sys; d=json.load(open('$ACCESS_FILE')); print(next(iter(d.get('groups',{})),''))" 2>/dev/null)
fi
[ -z "$CHANNEL_ID" ] && exit 0

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "https://discord.com/api/v10/channels/${CHANNEL_ID}/messages" \
  -H "Authorization: Bot ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"content\":\"[$(date '+%Y-%m-%d %H:%M:%S')] Claude has restarted.\"}")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
if [ "$HTTP_CODE" -lt 200 ] || [ "$HTTP_CODE" -ge 300 ]; then
  echo "[discord-restart-notify] Failed to send notification (HTTP $HTTP_CODE)" >&2
fi

exit 0

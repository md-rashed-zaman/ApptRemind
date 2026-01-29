#!/usr/bin/env bash
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

BASE_URL="${BASE_URL:-http://localhost}"
MAILPIT_API="${MAILPIT_API:-$BASE_URL:8025/api/v1/messages}"

APPOINTMENT_ID="${APPOINTMENT_ID:-$(python3 -c 'import uuid; print(uuid.uuid4())')}"
BUSINESS_ID="${BUSINESS_ID:-$(python3 -c 'import uuid; print(uuid.uuid4())')}"
RECIPIENT="${RECIPIENT:-notify-smoke-$(date +%s)@example.com}"

echo "Publishing reminder-requested -> scheduler -> notification -> mailpit"
echo "appointment_id=$APPOINTMENT_ID"
echo "business_id=$BUSINESS_ID"
echo "recipient=$RECIPIENT"

RECIPIENT="$RECIPIENT" APPOINTMENT_ID="$APPOINTMENT_ID" BUSINESS_ID="$BUSINESS_ID" ./scripts/publish-reminder-requested.sh >/dev/null

found=0
for _ in $(seq 1 30); do
  # Mailpit message schema is stable enough to match on the recipient address.
  if curl -sS "$MAILPIT_API" | jq -e --arg to "$RECIPIENT" '.messages[]? | select(.To[]?.Address == $to)' >/dev/null; then
    found=1
    break
  fi
  sleep 1
done

if [[ "$found" != "1" ]]; then
  echo "Timed out waiting for Mailpit email to $RECIPIENT"
  curl -sS "$MAILPIT_API" | jq '.messages | length' || true
  exit 1
fi

echo "Mailpit: email delivered"

# Optional: verify analytics consumer wrote a metric row (uses container psql; no host dependency).
if docker compose -f deploy/compose/docker-compose.yml ps analytics-service >/dev/null 2>&1; then
  docker compose -f deploy/compose/docker-compose.yml exec -T postgres psql -U analytics_user -d analytics_db -tAc \
    "SELECT status, count(*) FROM notification_metrics GROUP BY status ORDER BY status;" || true
fi

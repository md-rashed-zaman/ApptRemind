#!/usr/bin/env bash
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

BASE_URL="${BASE_URL:-http://localhost:8080}"
MAILPIT_API="${MAILPIT_API:-http://localhost:8025/api/v1/messages}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-90}"

echo "Running booking -> reminder -> scheduler -> notification -> mailpit smoke test"

wait_for() {
  local name="$1"
  local seconds="$2"
  shift 2
  for _ in $(seq 1 "$seconds"); do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "timed out waiting for $name" >&2
  return 1
}

wait_for "gateway /healthz" 30 curl -fsS "$BASE_URL/healthz"

echo "Ensuring migrations..."
make migrate-auth >/dev/null
make migrate-business >/dev/null
make migrate-booking >/dev/null
make migrate-scheduler >/dev/null
make migrate-notification >/dev/null
make migrate-analytics >/dev/null

curl -sS -X DELETE "http://localhost:8025/api/v1/messages" >/dev/null || true

EMAIL="owner-booking-$(date +%s)@example.com"
REG_JSON="$(
  curl -sS -X POST "$BASE_URL/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\",\"password\":\"pass123\",\"business_name\":\"Booking Smoke Biz\"}"
)"
TOKEN="$(echo "$REG_JSON" | jq -r .access_token)"
BUSINESS_ID="$(curl -sS "$BASE_URL/api/v1/auth/me" -H "Authorization: Bearer $TOKEN" | jq -r .business_id)"

echo "business_id=$BUSINESS_ID"

# Use UTC to keep date math simple and set a fast reminder policy (1 minute).
curl -sS -X PUT "$BASE_URL/api/v1/business/profile" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"timezone":"UTC","reminder_offsets_minutes":[1]}' >/dev/null

SERVICE_ID="$(
  curl -sS -X POST "$BASE_URL/api/v1/business/services" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Consult","duration_minutes":30,"price":10,"description":"smoke"}' | jq -r .id
)"

STAFF_ID="$(
  curl -sS -X POST "$BASE_URL/api/v1/business/staff" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Alice"}' | jq -r .id
)"

# Open all day today (UTC).
WEEKDAY="$(date -u +%w)" # 0=Sun..6=Sat
curl -sS -X PUT "$BASE_URL/api/v1/business/staff/working-hours?staff_id=$STAFF_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"weekday\":$WEEKDAY,\"is_working\":true,\"start_minute\":0,\"end_minute\":1440}" >/dev/null

RECIPIENT="customer-booking-$(date +%s)@example.com"

START_TIME="$(date -u -d '+70 seconds' +%Y-%m-%dT%H:%M:%SZ)"
END_TIME="$(date -u -d '+2500 seconds' +%Y-%m-%dT%H:%M:%SZ)" # 30m window from now+70s

BOOK_JSON="$(
  curl -sS -X POST "$BASE_URL/api/v1/public/book" \
    -H "Content-Type: application/json" \
    -d "{
      \"business_id\":\"$BUSINESS_ID\",
      \"service_id\":\"$SERVICE_ID\",
      \"staff_id\":\"$STAFF_ID\",
      \"customer_name\":\"Smoke Customer\",
      \"customer_email\":\"$RECIPIENT\",
      \"customer_phone\":\"\",
      \"start_time\":\"$START_TIME\",
      \"end_time\":\"$END_TIME\"
    }"
)"

APPOINTMENT_ID="$(echo "$BOOK_JSON" | jq -r .appointment_id)"
if [[ -z "$APPOINTMENT_ID" || "$APPOINTMENT_ID" == "null" ]]; then
  echo "booking failed: $BOOK_JSON"
  exit 1
fi
echo "appointment_id=$APPOINTMENT_ID"
echo "recipient=$RECIPIENT"
echo "start_time=$START_TIME"

echo "Waiting for email..."
found=0
for _ in $(seq 1 "$TIMEOUT_SECONDS"); do
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

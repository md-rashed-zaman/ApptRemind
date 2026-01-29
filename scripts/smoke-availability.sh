#!/usr/bin/env bash
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

BASE_URL="${BASE_URL:-http://localhost:8080}"
DATE="${DATE:-2026-01-28}"      # Wed
WEEKEND="${WEEKEND:-2026-02-01}" # Sun

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

EMAIL="owner-$(date +%s)@example.com"

REG_JSON="$(
  curl -sS -X POST "$BASE_URL/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\",\"password\":\"pass123\",\"business_name\":\"Smoke Biz\"}"
)"

TOKEN="$(echo "$REG_JSON" | jq -r .access_token)"
if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "failed to get access token: $REG_JSON"
  exit 1
fi

BUSINESS_ID="$(
  curl -sS "$BASE_URL/api/v1/auth/me" -H "Authorization: Bearer $TOKEN" | jq -r .business_id
)"

echo "business_id=$BUSINESS_ID"

curl -sS -X PUT "$BASE_URL/api/v1/business/profile" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Smoke Biz","timezone":"America/New_York"}' >/dev/null

SERVICE_ID="$(
  curl -sS -X POST "$BASE_URL/api/v1/business/services" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Consult","duration_minutes":25,"price":10,"description":"smoke"}' | jq -r .id
)"

STAFF_ID="$(
  curl -sS -X POST "$BASE_URL/api/v1/business/staff" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Alice"}' | jq -r .id
)"

echo "service_id=$SERVICE_ID"
echo "staff_id=$STAFF_ID"

echo
echo "weekday slots ($DATE):"
curl -sS "$BASE_URL/api/v1/public/slots?business_id=$BUSINESS_ID&staff_id=$STAFF_ID&service_id=$SERVICE_ID&date=$DATE" | jq '.[0:5]'

echo
echo "weekend slots ($WEEKEND) count (default schedule should be closed):"
DEFAULT_WEEKEND_COUNT="$(curl -sS "$BASE_URL/api/v1/public/slots?business_id=$BUSINESS_ID&staff_id=$STAFF_ID&service_id=$SERVICE_ID&date=$WEEKEND" | jq 'length')"
echo "$DEFAULT_WEEKEND_COUNT"
if [[ "$DEFAULT_WEEKEND_COUNT" != "0" ]]; then
  echo "expected 0 weekend slots by default"
  exit 1
fi

echo
echo "opening Sunday (weekday=0) 10:00-14:00 local time..."
curl -sS -X PUT "$BASE_URL/api/v1/business/staff/working-hours?staff_id=$STAFF_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"weekday":0,"is_working":true,"start_minute":600,"end_minute":840}' >/dev/null

echo "weekend slots ($WEEKEND) count (after update):"
UPDATED_WEEKEND_COUNT="$(curl -sS "$BASE_URL/api/v1/public/slots?business_id=$BUSINESS_ID&staff_id=$STAFF_ID&service_id=$SERVICE_ID&date=$WEEKEND" | jq 'length')"
echo "$UPDATED_WEEKEND_COUNT"
if [[ "$UPDATED_WEEKEND_COUNT" -le 0 ]]; then
  echo "expected weekend slots after opening Sunday"
  exit 1
fi

echo
echo "adding time off on Sunday (11:00-12:00 local time)..."
# In America/New_York on 2026-02-01, 11:00-12:00 local is 16:00-17:00 UTC.
TIMEOFF_ID="$(
  curl -sS -X POST "$BASE_URL/api/v1/business/staff/time-off?staff_id=$STAFF_ID" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"start_time":"2026-02-01T16:00:00Z","end_time":"2026-02-01T17:00:00Z","reason":"lunch"}' | jq -r .id
)"
if [[ -z "$TIMEOFF_ID" || "$TIMEOFF_ID" == "null" ]]; then
  echo "failed to create time off"
  exit 1
fi

echo "weekend slots ($WEEKEND) count (after time off):"
AFTER_TIMEOFF_COUNT="$(curl -sS "$BASE_URL/api/v1/public/slots?business_id=$BUSINESS_ID&staff_id=$STAFF_ID&service_id=$SERVICE_ID&date=$WEEKEND" | jq 'length')"
echo "$AFTER_TIMEOFF_COUNT"
if [[ "$AFTER_TIMEOFF_COUNT" -ge "$UPDATED_WEEKEND_COUNT" ]]; then
  echo "expected fewer slots after time off"
  exit 1
fi

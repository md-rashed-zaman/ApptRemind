#!/usr/bin/env bash
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

BASE_URL="${BASE_URL:-http://localhost:8080}"

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

EMAIL="owner-outside-$(date +%s)@example.com"
REG_JSON="$(
  curl -sS -X POST "$BASE_URL/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\",\"password\":\"pass123\",\"business_name\":\"Outside Smoke Biz\"}"
)"
TOKEN="$(echo "$REG_JSON" | jq -r .access_token)"
BUSINESS_ID="$(curl -sS "$BASE_URL/api/v1/auth/me" -H "Authorization: Bearer $TOKEN" | jq -r .business_id)"

curl -sS -X PUT "$BASE_URL/api/v1/business/profile" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"timezone":"UTC"}' >/dev/null

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

# Restrict today's working hours to 09:00-10:00 UTC.
WEEKDAY="$(date -u +%w)"
curl -sS -X PUT "$BASE_URL/api/v1/business/staff/working-hours?staff_id=$STAFF_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"weekday\":$WEEKDAY,\"is_working\":true,\"start_minute\":540,\"end_minute\":600}" >/dev/null

# Attempt to book at 12:00-12:30 UTC today -> must be rejected.
DATE="$(date -u +%Y-%m-%d)"
START_TIME="${DATE}T12:00:00Z"
END_TIME="${DATE}T12:30:00Z"

STATUS="$(
  curl -sS -o /tmp/apptremind_outside_booking_body.json -w '%{http_code}' \
    -X POST "$BASE_URL/api/v1/public/book" \
    -H "Content-Type: application/json" \
    -d "{
      \"business_id\":\"$BUSINESS_ID\",
      \"service_id\":\"$SERVICE_ID\",
      \"staff_id\":\"$STAFF_ID\",
      \"customer_name\":\"Smoke Customer\",
      \"customer_email\":\"outside-$(date +%s)@example.com\",
      \"customer_phone\":\"\",
      \"start_time\":\"$START_TIME\",
      \"end_time\":\"$END_TIME\"
    }"
)"

echo "status=$STATUS"
cat /tmp/apptremind_outside_booking_body.json
echo

if [[ "$STATUS" != "422" ]]; then
  echo "expected HTTP 422"
  exit 1
fi

echo "OK: booking outside availability rejected"

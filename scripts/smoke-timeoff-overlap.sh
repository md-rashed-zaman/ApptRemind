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

echo "Ensuring business migrations..."
make migrate-business >/dev/null

EMAIL="owner-timeoff-$(date +%s)@example.com"
REG_JSON="$(
  curl -sS -X POST "$BASE_URL/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\",\"password\":\"pass123\",\"business_name\":\"TimeOff Smoke Biz\"}"
)"
TOKEN="$(echo "$REG_JSON" | jq -r .access_token)"

curl -sS -X PUT "$BASE_URL/api/v1/business/profile" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"timezone":"UTC"}' >/dev/null

STAFF_ID="$(
  curl -sS -X POST "$BASE_URL/api/v1/business/staff" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name":"Alice"}' | jq -r .id
)"

echo "staff_id=$STAFF_ID"

STATUS1="$(
  curl -sS -o /tmp/apptremind_timeoff1.json -w '%{http_code}' \
    -X POST "$BASE_URL/api/v1/business/staff/time-off?staff_id=$STAFF_ID" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"start_time":"2026-02-01T10:00:00Z","end_time":"2026-02-01T11:00:00Z","reason":"block1"}'
)"
echo "create1 status=$STATUS1"

STATUS2="$(
  curl -sS -o /tmp/apptremind_timeoff2.json -w '%{http_code}' \
    -X POST "$BASE_URL/api/v1/business/staff/time-off?staff_id=$STAFF_ID" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"start_time":"2026-02-01T10:30:00Z","end_time":"2026-02-01T11:30:00Z","reason":"overlap"}'
)"
echo "create2 status=$STATUS2"
cat /tmp/apptremind_timeoff2.json
echo

if [[ "$STATUS1" != "201" ]]; then
  echo "expected first create to be 201"
  cat /tmp/apptremind_timeoff1.json || true
  exit 1
fi
if [[ "$STATUS2" != "409" ]]; then
  echo "expected overlapping create to be 409"
  exit 1
fi

echo "OK: overlap rejected"

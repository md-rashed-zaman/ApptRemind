#!/usr/bin/env bash
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

BASE_URL="${BASE_URL:-http://localhost:8080}"

if [[ -z "${STRIPE_WEBHOOK_SECRET:-}" ]]; then
  echo "STRIPE_WEBHOOK_SECRET is required (example: whsec_...)"
  echo "Tip: export STRIPE_WEBHOOK_SECRET=whsec_test and restart billing-service/gateway-service"
  exit 1
fi

echo "Running billing Stripe webhook smoke test (signature verification + idempotency)"

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
make migrate-billing >/dev/null

EMAIL="owner-stripe-$(date +%s)@example.com"
REG_JSON="$(
  curl -sS -X POST "$BASE_URL/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$EMAIL\",\"password\":\"pass123\",\"business_name\":\"Stripe Smoke Biz\"}"
)"
TOKEN="$(echo "$REG_JSON" | jq -r .access_token)"
BUSINESS_ID="$(curl -sS "$BASE_URL/api/v1/auth/me" -H "Authorization: Bearer $TOKEN" | jq -r .business_id)"

echo "business_id=$BUSINESS_ID"

# Send a signed webhook event through the gateway. This endpoint is intentionally not JWT protected.
BUSINESS_ID="$BUSINESS_ID" TIER="starter" BASE_URL="$BASE_URL" STRIPE_WEBHOOK_SECRET="$STRIPE_WEBHOOK_SECRET" \
  go run ./tools/stripe-webhook-sim -type checkout.session.completed >/dev/null

echo "OK: Stripe webhook accepted"


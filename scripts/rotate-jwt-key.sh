#!/usr/bin/env bash
set -euo pipefail

JWKS_URL="${JWKS_URL:-http://localhost:8080/.well-known/jwks.json}"
ROTATE_URL="${ROTATE_URL:-http://localhost:8080/api/v1/auth/rotate}"
ROTATE_KEY="${ROTATE_KEY:-}"

if [[ -z "$ROTATE_KEY" ]]; then
  echo "ROTATE_KEY is required"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required"
  exit 1
fi

current="$(curl -sS "$JWKS_URL" | jq -r '.keys[0].kid // empty')"
if [[ -z "$current" ]]; then
  echo "No kid found in JWKS"
  exit 1
fi

next="$(curl -sS "$JWKS_URL" | jq -r --arg cur "$current" '.keys[].kid | select(. != $cur) | head -n 1')"
if [[ -z "$next" ]]; then
  echo "No alternate kid found; add another key to JWKS first"
  exit 1
fi

echo "Rotating active kid: $current -> $next"

curl -sS -X POST "$ROTATE_URL" \
  -H "Content-Type: application/json" \
  -H "X-Rotate-Key: $ROTATE_KEY" \
  -d "{\"active_kid\":\"$next\"}"

echo "Rotation request sent."

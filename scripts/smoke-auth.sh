#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd jq

email="smoke-$(date +%s)-${RANDOM}@example.com"
password="pass123"
business="Smoke Test Biz"

register_body="$(mktemp)"
refresh_body="$(mktemp)"
logout_body="$(mktemp)"
trap 'rm -f "$register_body" "$refresh_body" "$logout_body"' EXIT

echo "==> Registering $email"
register_code="$(
  curl -sS -o "$register_body" -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\",\"password\":\"$password\",\"business_name\":\"$business\"}"
)"

if [[ "$register_code" != "201" ]]; then
  echo "register failed (status $register_code):"
  cat "$register_body"
  exit 1
fi

access_token="$(jq -r '.access_token // empty' <"$register_body")"
refresh_token="$(jq -r '.refresh_token // empty' <"$register_body")"

if [[ -z "$access_token" || -z "$refresh_token" ]]; then
  echo "register response missing tokens:"
  cat "$register_body"
  exit 1
fi

echo "==> Refreshing tokens"
refresh_code="$(
  curl -sS -o "$refresh_body" -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/auth/refresh" \
    -H "Content-Type: application/json" \
    -d "{\"refresh_token\":\"$refresh_token\"}"
)"

if [[ "$refresh_code" != "200" ]]; then
  echo "refresh failed (status $refresh_code):"
  cat "$refresh_body"
  exit 1
fi

new_access_token="$(jq -r '.access_token // empty' <"$refresh_body")"
new_refresh_token="$(jq -r '.refresh_token // empty' <"$refresh_body")"

if [[ -z "$new_access_token" || -z "$new_refresh_token" ]]; then
  echo "refresh response missing tokens:"
  cat "$refresh_body"
  exit 1
fi

echo "==> Logging out"
logout_code="$(
  curl -sS -o "$logout_body" -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/auth/logout" \
    -H "Content-Type: application/json" \
    -d "{\"refresh_token\":\"$new_refresh_token\"}"
)"

if [[ "$logout_code" != "204" ]]; then
  echo "logout failed (status $logout_code):"
  cat "$logout_body"
  exit 1
fi

echo "==> Verifying revoked refresh token is rejected"
post_logout_code="$(
  curl -sS -o /dev/null -w "%{http_code}" \
    -X POST "$BASE_URL/api/v1/auth/refresh" \
    -H "Content-Type: application/json" \
    -d "{\"refresh_token\":\"$new_refresh_token\"}"
)"

if [[ "$post_logout_code" != "401" ]]; then
  echo "expected 401 after logout, got $post_logout_code"
  exit 1
fi

echo "Auth smoke test passed."

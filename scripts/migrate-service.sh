#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/deploy/compose/docker-compose.yml}"

SERVICE_NAME="${SERVICE_NAME:-}"
DB_USER="${DB_USER:-}"
DB_NAME="${DB_NAME:-}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-}"

if [[ -z "$SERVICE_NAME" || -z "$DB_USER" || -z "$DB_NAME" || -z "$MIGRATIONS_DIR" ]]; then
  echo "SERVICE_NAME, DB_USER, DB_NAME, and MIGRATIONS_DIR are required" >&2
  exit 1
fi

if [[ ! -d "$MIGRATIONS_DIR" ]]; then
  echo "Migrations dir not found: $MIGRATIONS_DIR" >&2
  exit 1
fi

use_docker_psql=false
if ! command -v psql >/dev/null 2>&1; then
  use_docker_psql=true
fi

run_psql_stdin() {
  if [[ "$use_docker_psql" == "true" ]]; then
    docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1
    return
  fi
  if [[ -z "${DATABASE_URL:-}" ]]; then
    echo "DATABASE_URL is required when running migrations with host psql." >&2
    exit 1
  fi
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1
}

run_psql_cmd() {
  local sql="$1"
  if [[ "$use_docker_psql" == "true" ]]; then
    docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 -c "$sql"
    return
  fi
  if [[ -z "${DATABASE_URL:-}" ]]; then
    echo "DATABASE_URL is required when running migrations with host psql." >&2
    exit 1
  fi
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "$sql"
}

run_psql_stdin <<'EOSQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
EOSQL

for file in "$MIGRATIONS_DIR"/*.sql; do
  version="$(basename "$file")"
  if [[ "$use_docker_psql" == "true" ]]; then
    applied="$(docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -tAc "SELECT 1 FROM schema_migrations WHERE version = '$version'")"
  else
    applied="$(psql "$DATABASE_URL" -tAc "SELECT 1 FROM schema_migrations WHERE version = '$version'")"
  fi
  if [[ "$applied" == "1" ]]; then
    echo "Skipping $version (already applied)"
    continue
  fi

  echo "Applying $version"
  if [[ "$use_docker_psql" == "true" ]]; then
    docker compose -f "$COMPOSE_FILE" exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 < "$file"
  else
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$file"
  fi
  run_psql_cmd "INSERT INTO schema_migrations (version) VALUES ('$version')"
done

echo "$SERVICE_NAME migrations complete."

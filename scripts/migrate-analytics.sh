#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export SERVICE_NAME="analytics-service"
export DB_USER="analytics_user"
export DB_NAME="analytics_db"
export MIGRATIONS_DIR="$ROOT_DIR/services/analytics-service/migrations"

exec "$ROOT_DIR/scripts/migrate-service.sh"

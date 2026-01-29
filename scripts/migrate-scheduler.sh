#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export SERVICE_NAME="scheduler-service"
export DB_USER="scheduler_user"
export DB_NAME="scheduler_db"
export MIGRATIONS_DIR="$ROOT_DIR/services/scheduler-service/migrations"

exec "$ROOT_DIR/scripts/migrate-service.sh"

#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export SERVICE_NAME="notification-service"
export DB_USER="notification_user"
export DB_NAME="notification_db"
export MIGRATIONS_DIR="$ROOT_DIR/services/notification-service/migrations"

exec "$ROOT_DIR/scripts/migrate-service.sh"

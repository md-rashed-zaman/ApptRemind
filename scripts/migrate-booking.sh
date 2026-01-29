#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export SERVICE_NAME="booking-service"
export DB_USER="booking_user"
export DB_NAME="booking_db"
export MIGRATIONS_DIR="$ROOT_DIR/services/booking-service/migrations"

exec "$ROOT_DIR/scripts/migrate-service.sh"

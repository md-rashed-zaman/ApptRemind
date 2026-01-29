#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export SERVICE_NAME="billing-service"
export DB_USER="billing_user"
export DB_NAME="billing_db"
export MIGRATIONS_DIR="$ROOT_DIR/services/billing-service/migrations"

exec "$ROOT_DIR/scripts/migrate-service.sh"

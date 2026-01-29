#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export SERVICE_NAME="business-service"
export DB_USER="business_user"
export DB_NAME="business_db"
export MIGRATIONS_DIR="$ROOT_DIR/services/business-service/migrations"

exec "$ROOT_DIR/scripts/migrate-service.sh"

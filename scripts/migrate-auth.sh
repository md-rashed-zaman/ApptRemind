#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export SERVICE_NAME="auth-service"
export DB_USER="auth_user"
export DB_NAME="auth_db"
export MIGRATIONS_DIR="$ROOT_DIR/services/auth-service/migrations"

exec "$ROOT_DIR/scripts/migrate-service.sh"

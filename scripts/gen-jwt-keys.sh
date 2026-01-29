#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${1:-/tmp/apptremind-jwt}"
mkdir -p "$OUT_DIR"

PRIVATE_KEY="$OUT_DIR/jwt_private.pem"
PUBLIC_KEY="$OUT_DIR/jwt_public.pem"

openssl genrsa -out "$PRIVATE_KEY" 2048 >/dev/null 2>&1
openssl rsa -in "$PRIVATE_KEY" -pubout -out "$PUBLIC_KEY" >/dev/null 2>&1

echo "Generated:"
echo "  $PRIVATE_KEY"
echo "  $PUBLIC_KEY"

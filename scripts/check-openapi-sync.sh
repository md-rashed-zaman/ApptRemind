#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OPENAPI="$ROOT_DIR/openapi/gateway.v1.yaml"
ASSET="$ROOT_DIR/services/gateway-service/cmd/gateway-service/assets/gateway.v1.yaml"

if [[ ! -f "$OPENAPI" || ! -f "$ASSET" ]]; then
  echo "OpenAPI files not found" >&2
  exit 1
fi

python3 - <<PY
import sys, yaml, pathlib
openapi = pathlib.Path("$OPENAPI")
asset = pathlib.Path("$ASSET")
for path in (openapi, asset):
    with path.open("r", encoding="utf-8") as f:
        yaml.safe_load(f)
print("OpenAPI YAML parses ok")
PY

if ! diff -u "$OPENAPI" "$ASSET" >/dev/null; then
  echo "OpenAPI specs are out of sync. Please update both files." >&2
  diff -u "$OPENAPI" "$ASSET" | head -n 200
  exit 1
fi

echo "OpenAPI specs are in sync."

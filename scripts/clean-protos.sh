#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LEGACY_DIR="$ROOT_DIR/protos/gen/github.com"

if [[ -d "$LEGACY_DIR" ]]; then
  rm -rf "$LEGACY_DIR"
  echo "Removed legacy generated protos at $LEGACY_DIR"
else
  echo "No legacy generated protos found."
fi

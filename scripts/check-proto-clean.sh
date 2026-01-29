#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

make proto >/dev/null

if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  if ! git diff --exit-code -- protos/gen; then
    echo "Generated protos are out of date. Run 'make proto' and commit changes." >&2
    exit 1
  fi
else
  echo "git repo not available; skipping proto diff check."
fi

echo "Proto generation is clean."

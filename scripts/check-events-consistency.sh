#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export ROOT_DIR
DOCS_EVENTS="$ROOT_DIR/docs/events.md"
COMPOSE="$ROOT_DIR/deploy/compose/docker-compose.yml"

if [[ ! -f "$DOCS_EVENTS" || ! -f "$COMPOSE" ]]; then
  echo "Missing docs/events.md or docker-compose.yml" >&2
  exit 1
fi

python3 - <<'PY'
import os, re
from pathlib import Path

root = Path(os.environ.get("ROOT_DIR", ".")).resolve()
docs_path = root / "docs" / "events.md"
compose_path = root / "deploy" / "compose" / "docker-compose.yml"

docs = docs_path.read_text(encoding="utf-8")
compose = compose_path.read_text(encoding="utf-8")

doc_events = set(re.findall(r"^- event:\\s+([\\w\\.\\-]+)$", docs, flags=re.MULTILINE))
compose_events = set(re.findall(r"--topic\\s+([\\w\\.\\-]+)", compose))

missing_in_docs = sorted(compose_events - doc_events)
missing_in_compose = sorted(doc_events - compose_events)

if missing_in_docs:
    print("Events in compose but missing in docs:")
    for e in missing_in_docs:
        print(f" - {e}")
    raise SystemExit(1)

if missing_in_compose:
    print("Events in docs but missing in compose topics:")
    for e in missing_in_compose:
        print(f" - {e}")
    raise SystemExit(1)

print("Event catalog matches compose topics.")
PY

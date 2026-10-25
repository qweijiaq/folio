#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$ROOT/.run"
rm -f "$ROOT/.run/e2e.db" "$ROOT/.run/e2e.db-shm" "$ROOT/.run/e2e.db-wal"

cd "$ROOT/server"
export FOLIO_DB_DRIVER=sqlite
export FOLIO_DB="$ROOT/.run/e2e.db"
export FOLIO_DEMO=1
export FOLIO_SECRET=e2e-test-secret
export FOLIO_ADDR=127.0.0.1:8091
export FOLIO_WEB="$ROOT/web/dist"
export FOLIO_UPLOAD_DIR="$ROOT/.run/e2e-uploads"
export FOLIO_REDIS_ADDR=""
exec go run .

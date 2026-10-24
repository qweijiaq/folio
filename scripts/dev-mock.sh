#!/usr/bin/env bash
# 完全无外部依赖的体验模式：进程内 SQLite + 演示数据 + 前端开发服务器。
set -euo pipefail
cd "$(dirname "$0")/.."

export FOLIO_DB_DRIVER=memory
export FOLIO_DEMO=1
export FOLIO_REDIS_ADDR=""
export FOLIO_NO_ENV=1

exec ./dev.sh

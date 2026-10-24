#!/usr/bin/env bash
# 使用已启动的 Docker MySQL + Redis 做本地开发。
# 先运行：docker compose up -d db redis
set -euo pipefail
cd "$(dirname "$0")/.."

export FOLIO_DB_DRIVER="${FOLIO_DB_DRIVER:-mysql}"
export FOLIO_MYSQL_DSN="${FOLIO_MYSQL_DSN:-folio:folio_pass@tcp(127.0.0.1:3306)/folio?charset=utf8mb4&parseTime=True&loc=Local}"
export FOLIO_REDIS_ADDR="${FOLIO_REDIS_ADDR:-127.0.0.1:6379}"
export FOLIO_DEMO="${FOLIO_DEMO:-1}"
export FOLIO_NO_ENV=1

exec ./dev.sh

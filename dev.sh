#!/usr/bin/env bash
# 一键启动开发环境：后端 :8080 + 前端 :5173
set -euo pipefail

cd "$(dirname "$0")"

if [ "${FOLIO_NO_ENV:-0}" != "1" ] && [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

if [ ! -d web/node_modules ]; then
  echo "首次运行，安装前端依赖…"
  (cd web && npm install)
fi

if [ ! -f server/data/folio.db ] && [ "${FOLIO_DB_DRIVER:-sqlite}" = "sqlite" ]; then
  echo "首次启动会写入演示内容（可用 FOLIO_DEMO=0 关闭）。"
fi

if [ "${FOLIO_ADDR:-:8080}" = ":8080" ] && lsof -nP -iTCP:8080 -sTCP:LISTEN >/dev/null 2>&1; then
  echo "端口 8080 已被占用（可能 Docker 应用正在运行）。"
  echo "先执行 docker compose stop app，或设置 FOLIO_ADDR 使用其他端口。"
  exit 1
fi

echo "数据库模式：${FOLIO_DB_DRIVER:-sqlite}　Redis：${FOLIO_REDIS_ADDR:-关闭}"

(cd server && FOLIO_DEMO="${FOLIO_DEMO:-1}" go run .) &
SERVER_PID=$!

cleanup() {
  kill "$SERVER_PID" 2>/dev/null || true
  pkill -P "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo
echo "后端：http://localhost:8080"
echo "前端：http://localhost:5173"
echo

cd web
npm run dev

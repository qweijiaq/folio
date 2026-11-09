#!/usr/bin/env bash
# 生产服务器上一键更新：拉取代码、重建应用镜像、滚动更新容器。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [ ! -f .env ]; then
  echo "缺少 .env，请先按 README 配置生产环境变量。" >&2
  exit 1
fi

echo "拉取最新代码…"
git pull --ff-only

echo "构建应用镜像…"
docker compose build app

echo "更新容器…"
docker compose -f docker-compose.yml -f docker-compose.edge.yml up -d

echo "清理旧镜像…"
docker image prune -f >/dev/null

echo "完成。"
docker compose -f docker-compose.yml -f docker-compose.edge.yml ps

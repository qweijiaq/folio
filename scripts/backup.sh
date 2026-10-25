#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

DEST="${BACKUP_DIR:-$ROOT/backups}"
KEEP_DAYS="${BACKUP_KEEP_DAYS:-14}"
STAMP="$(date +%Y%m%d-%H%M%S)"
mkdir -p "$DEST"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "导出 MySQL…"
docker compose exec -T db sh -c 'exec mysqldump -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" --single-transaction --routines --triggers --no-tablespaces "$MYSQL_DATABASE"' > "$TMP/folio.sql"

echo "复制上传目录…"
if ! docker compose cp app:/app/data/uploads "$TMP/uploads" >/dev/null 2>&1; then
  mkdir -p "$TMP/uploads"
  echo "提示：没有读取到本地上传目录（可能使用七牛云存储）。"
fi

ARCHIVE="$DEST/folio-$STAMP.tar.gz"
tar -czf "$ARCHIVE" -C "$TMP" folio.sql uploads
ln -sfn "$(basename "$ARCHIVE")" "$DEST/latest.tar.gz"

if [[ "$KEEP_DAYS" =~ ^[0-9]+$ ]] && (( KEEP_DAYS > 0 )); then
  find "$DEST" -maxdepth 1 -type f -name 'folio-*.tar.gz' -mtime +"$KEEP_DAYS" -delete
fi

echo "备份完成：$ARCHIVE"
echo "恢复数据库：tar -xzf <备份> -C /tmp && docker compose exec -T db sh -c 'exec mysql -u\"\$MYSQL_USER\" -p\"\$MYSQL_PASSWORD\" \"\$MYSQL_DATABASE\"' < /tmp/folio.sql"
echo "恢复上传目录：docker compose cp /tmp/uploads/. app:/app/data/uploads/"

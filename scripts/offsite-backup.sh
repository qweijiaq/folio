#!/usr/bin/env bash
# 把最近一次本地备份加密后上传到阿里云 OSS。
# 依赖 aliyun CLI（已配置 AK 或 ECS RAM 角色）和 openssl。
set -euo pipefail

# systemd oneshot 环境里可能没有 HOME，aliyun CLI 会因此找不到 /root/.aliyun/config.json。
# 这里显式兜底，保证通过 systemd、cron 或手动执行时行为一致。
export HOME="${HOME:-/root}"
export PATH="${PATH:-/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [ ! -f .env ]; then
  echo "缺少 .env" >&2
  exit 1
fi

get_env() {
  grep -E "^$1=" .env | tail -n 1 | cut -d= -f2- | tr -d '\r'
}

OSS_BUCKET="$(get_env OSS_BUCKET)"
OSS_PREFIX="$(get_env OSS_PREFIX)"
OSS_REGION="$(get_env OSS_REGION)"
PASSPHRASE_FILE="$(get_env OSS_BACKUP_PASSPHRASE_FILE)"
OSS_PREFIX="${OSS_PREFIX:-folio-backups}"
OSS_PREFIX="${OSS_PREFIX#/}"
OSS_PREFIX="${OSS_PREFIX%/}"
PASSPHRASE_FILE="${PASSPHRASE_FILE:-/root/.folio-backup-pass}"

if [ -z "$OSS_BUCKET" ]; then
  echo "未配置 OSS_BUCKET，异地备份未启用。" >&2
  echo "请先在 .env 设置 OSS_BUCKET / OSS_REGION，并配置 aliyun CLI。" >&2
  exit 1
fi

command -v aliyun >/dev/null 2>&1 || {
  echo "未找到 aliyun CLI，请先安装并配置。" >&2
  exit 1
}

echo "生成本地备份…"
"$ROOT/scripts/backup.sh"

LATEST="$ROOT/backups/latest.tar.gz"
if [ ! -e "$LATEST" ]; then
  echo "没有找到 $LATEST" >&2
  exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

UPLOAD="$LATEST"
REMOTE_NAME="$(basename "$(readlink -f "$LATEST")")"
if [ -f "$PASSPHRASE_FILE" ]; then
  UPLOAD="$TMP/$REMOTE_NAME.enc"
  REMOTE_NAME="$REMOTE_NAME.enc"
  echo "加密备份…"
  openssl enc -aes-256-cbc -pbkdf2 -salt \
    -in "$LATEST" -out "$UPLOAD" -pass "file:$PASSPHRASE_FILE"
else
  echo "警告：没有找到 $PASSPHRASE_FILE，将上传未加密备份。" >&2
fi

REMOTE="oss://$OSS_BUCKET/$OSS_PREFIX/$REMOTE_NAME"
echo "上传到 $REMOTE …"
if [ -n "$OSS_REGION" ]; then
  aliyun oss cp "$UPLOAD" "$REMOTE" --force --region "$OSS_REGION"
else
  aliyun oss cp "$UPLOAD" "$REMOTE" --force
fi

mkdir -p /var/lib/folio-offsite
cat > /var/lib/folio-offsite/last-success <<EOF
{"timestamp": $(date +%s), "remote": "$REMOTE", "size": $(stat -c%s "$UPLOAD")}
EOF
chmod 600 /var/lib/folio-offsite/last-success
echo "异地备份完成：$REMOTE"

#!/usr/bin/env bash
# 恢复：把 backup.sh 产出的备份还原回运行中的栈。
#
# 用法：
#   ./scripts/restore.sh backups/db-20260101-120000.sql.gz
#
# 危险操作：会清空 public schema 后重建。必须显式输入库名二次确认，
# 避免手滑把备份恢复到错误的库上。
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "用法: $0 <db-YYYYmmdd-HHMMSS.sql.gz>" >&2
  exit 1
fi
DUMP="$1"
[ -f "$DUMP" ] || { echo "找不到备份文件: $DUMP" >&2; exit 1; }

DB_USER="${DB_USER:-login_user}"
DB_NAME="${DB_NAME:-login_system}"
DIR="$(cd "$(dirname "$DUMP")" && pwd)"
BASE="$(basename "$DUMP")"
TS="${BASE#db-}"; TS="${TS%.sql.gz}"

echo "即将把 $BASE 恢复到数据库 $DB_NAME（会清空 public schema）。"
printf '请输入库名以确认: '
read -r confirm
if [ "$confirm" != "$DB_NAME" ]; then
  echo "输入不匹配，已取消。"
  exit 1
fi

echo "1/3 重建 schema"
docker compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 \
  -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'

echo "2/3 导入数据"
gunzip -c "$DIR/$BASE" \
  | docker compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1

echo "3/3 还原文件卷（如备份中存在）"
for vol in uploads drive; do
  arc="$DIR/$vol-$TS.tgz"
  if [ -f "$arc" ]; then
    docker compose run --rm --no-deps \
      -v "$DIR:/backup" \
      backend tar xzf "/backup/$vol-$TS.tgz" -C /data
    echo "  卷 $vol 已还原"
  else
    echo "  跳过 $vol（备份中不存在 $vol-$TS.tgz）"
  fi
done

echo "恢复完成。建议重启后端以重置缓存：docker compose restart backend"

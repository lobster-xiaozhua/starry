#!/usr/bin/env bash
# 备份：数据库（pg_dump）+ 上传附件卷 + 网盘卷。
#
# 用法：
#   ./scripts/backup.sh                 # 输出到 ./backups
#   BACKUP_DIR=/mnt/nas ./scripts/backup.sh
#   KEEP=30 ./scripts/backup.sh         # 只保留最近 30 份数据库备份
#
# 说明：数据卷的备份通过「临时起一个 backend 容器挂载同名卷 + tar」完成，
# 不需要在宿主机上知道卷的真实路径（不同 docker 版本的卷目录布局并不一致）。
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-backups}"
KEEP="${KEEP:-7}"
DB_USER="${DB_USER:-login_user}"
DB_NAME="${DB_NAME:-login_system}"

mkdir -p "$BACKUP_DIR"
BACKUP_DIR="$(cd "$BACKUP_DIR" && pwd)"
TS="$(date +%Y%m%d-%H%M%S)"

echo "备份到: $BACKUP_DIR"

# 1) 数据库：custom 格式交给 pg_dump 压缩，便于 pg_restore 选择性恢复。
docker compose exec -T postgres pg_dump -U "$DB_USER" -d "$DB_NAME" \
  | gzip > "$BACKUP_DIR/db-$TS.sql.gz"
echo "  数据库 -> db-$TS.sql.gz"

# 2) 文件卷：uploads（笔记附件）与 drive（网盘）
for vol in uploads drive; do
  docker compose run --rm --no-deps \
    -v "$BACKUP_DIR:/backup" \
    backend tar czf "/backup/$vol-$TS.tgz" -C /data "$vol" >/dev/null
  echo "  卷 $vol -> $vol-$TS.tgz"
done

# 3) 保留策略：只清理数据库备份，卷备份按同名时间戳成对保留，
#    统一按 db-* 的数量裁剪，避免留下无人认领的孤儿文件。
ls -1t "$BACKUP_DIR"/db-*.sql.gz 2>/dev/null | tail -n "+$((KEEP + 1))" | while read -r old; do
  base="$(basename "$old")"
  ts="${base#db-}"; ts="${ts%.sql.gz}"
  rm -f "$old" "$BACKUP_DIR/uploads-$ts.tgz" "$BACKUP_DIR/drive-$ts.tgz"
  echo "  已清理过期备份: $ts"
done

echo "完成。"

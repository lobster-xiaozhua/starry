#!/usr/bin/env bash
# 架构守卫：业务模块之间禁止横向依赖。
#
# 规则：
#   1. internal/modules/<a>/** 只允许 import 基础设施包（core/middleware/model/store/config/service/sse
#      以及标准库、第三方库），不允许 import 另一个业务模块 internal/modules/<b>/...
#   2. 业务模块不允许直接 import internal/handler（handler 层已下沉到各模块）。
#
# 失败时以非零状态退出，供 CI 门禁使用。
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODULES_DIR="$ROOT/backend/internal/modules"

if [ ! -d "$MODULES_DIR" ]; then
  echo "未找到模块目录: $MODULES_DIR" >&2
  exit 1
fi

MODULE_PREFIX="starry/backend/internal/modules/"
# 允许依赖的基础设施包前缀
ALLOWED_PREFIXES=(
  "starry/backend/internal/core"
  "starry/backend/internal/middleware"
  "starry/backend/internal/model"
  "starry/backend/internal/store"
  "starry/backend/internal/config"
  "starry/backend/internal/service"
  "starry/backend/internal/sse"
  "starry/backend/internal/authpkg"
  "starry/backend/internal/logx"
)

violations=0

for dir in "$MODULES_DIR"/*/; do
  name="$(basename "$dir")"
  [ -d "$dir" ] || continue

  while IFS= read -r file; do
    while IFS= read -r imp; do
      # 非本项目内部包：放行
      case "$imp" in
        "$MODULE_PREFIX"*) ;;
        "starry/backend/internal/handler"*)
          echo "违规: $file -> $imp (业务模块不得依赖已废弃的 internal/handler)"
          violations=$((violations + 1))
          continue
          ;;
        "starry/backend/internal/"*)
          allowed=0
          for p in "${ALLOWED_PREFIXES[@]}"; do
            case "$imp" in
              "$p"*) allowed=1; break ;;
            esac
          done
          if [ "$allowed" -eq 0 ]; then
            echo "违规: $file -> $imp (只允许依赖基础设施包)"
            violations=$((violations + 1))
          fi
          continue
          ;;
        *) continue ;;
      esac

      # 本模块内部 import：放行
      case "$imp" in
        "$MODULE_PREFIX$name/"*) continue ;;
      esac

      # 跨模块 import：违规
      echo "违规: $file -> $imp (模块 $name 不得依赖其它业务模块)"
      violations=$((violations + 1))
    done < <(grep -oE '"starry/backend/internal/[^"]+"' "$file" | tr -d '"' | sort -u)
  done < <(find "$dir" -name '*.go' -type f)
done

if [ "$violations" -ne 0 ]; then
  echo "" >&2
  echo "共发现 $violations 处模块隔离违规。" >&2
  exit 1
fi

echo "模块隔离检查通过：$(find "$MODULES_DIR" -mindepth 1 -maxdepth 1 -type d | wc -l) 个模块无横向依赖。"

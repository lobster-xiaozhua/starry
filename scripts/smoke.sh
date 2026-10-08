#!/usr/bin/env bash
# 端到端冒烟：对「已经跑起来的」整套服务做一次真实链路验证。
#
# 单元测试保证模块内部正确，冒烟保证「装配起来之后仍然正确」——
# 中间件顺序、模块开关、数据库迁移、限流、CORS 这些跨模块契约只有跑起来才验得到。
#
# 用法：
#   docker compose up -d --build          # 先起栈
#   ADMIN_PASSWORD=xxx ./scripts/smoke.sh
#
# 可选环境变量：
#   BASE_URL         默认 http://127.0.0.1:8080
#   ADMIN_USERNAME   默认 admin
#   ADMIN_PASSWORD   默认 admin123（compose 未设置时需在后端日志里找随机密码）
#   VERBOSE=1        打印每个用例的响应体
set -uo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
USERNAME="${ADMIN_USERNAME:-admin}"
PASSWORD="${ADMIN_PASSWORD:-admin123}"
VERBOSE="${VERBOSE:-0}"

TAB=$(printf '\t')
PASS=0
FAIL=0
FAILED_CASES=()

# jq 可用则用 jq，否则退回 python3，保证脚本在精简镜像里也能跑。
# 注意：check 用 bash -c 起子 shell 执行断言，因此函数必须导出，否则子 shell 里找不到。
if command -v jq >/dev/null 2>&1; then
  jget() { jq -r "$1" 2>/dev/null; }
else
  jget() { python3 -c '
import json,sys
expr=sys.argv[1]; data=sys.stdin.read()
try:
    obj=json.loads(data)
except Exception:
    print(""); sys.exit(0)
for part in expr.strip(".").split("."):
    if not part: continue
    if isinstance(obj, dict):
        obj=obj.get(part)
    else:
        obj=None
print("" if obj is None else obj)
' "$1"; }
fi
export -f jget 2>/dev/null || true

# check <用例名> <期望: 通过则返回0的 shell 表达式>
check() {
  local name="$1"
  shift
  if "$@"; then
    PASS=$((PASS + 1))
    printf '  \033[32mPASS\033[0m %s\n' "$name"
  else
    FAIL=$((FAIL + 1))
    FAILED_CASES+=("$name")
    printf '  \033[31mFAIL\033[0m %s\n' "$name"
  fi
}

# http <method> <path> [json] [extra curl args...] → 输出 "状态码\t响应体"
http() {
  local method="$1" path="$2" body="${3:-}"
  shift 3 2>/dev/null || shift $#
  local args=(-s -S -X "$method" "$BASE_URL$path" -w '\n%{http_code}')
  if [ -n "$body" ]; then
    args+=(-H 'content-type: application/json' -d "$body")
  fi
  args+=("$@")
  local out
  out="$(curl "${args[@]}" 2>/dev/null)"
  local code="${out##*$'\n'}"
  local resp="${out%$'\n'*}"
  printf '%s\t%s' "$code" "$resp"
}

# headers <path> [extra args] → 输出响应头
headers() {
  curl -s -S -D - -o /dev/null "$BASE_URL$1" "${@:2}" 2>/dev/null
}

echo "冒烟目标: $BASE_URL"
echo

# 等后端就绪：容器刚起来时数据库还在迁移，直接发请求会误报失败。
for _ in $(seq 1 60); do
  if curl -fsS "$BASE_URL/health" 2>/dev/null | grep -q '"status":"ok"'; then
    break
  fi
  sleep 2
done

# ---- 1. 存活与依赖 ----
health="$(curl -s -S "$BASE_URL/health" 2>/dev/null)"
check "GET /health 返回 200 且 status=ok" bash -c "
  [ \"\$(printf '%s' '$health' | jget '.status')\" = 'ok' ]"

# ---- 2. 未认证访问受保护端点 ----
unauth="$(http GET /api/admin/users)"
unauth_code="${unauth%%$TAB*}"
check "未带令牌访问管理端点返回 401" bash -c "[ '$unauth_code' = '401' ]"

# ---- 3. 登录 ----
login_body="$(printf '{"username":"%s","password":"%s"}' "$USERNAME" "$PASSWORD")"
login="$(http POST /api/auth/login "$login_body")"
login_code="${login%%$TAB*}"
login_resp="${login#*$TAB}"
if [ "$VERBOSE" = "1" ]; then echo "    login -> $login_resp"; fi
TOKEN="$(printf '%s' "$login_resp" | jget '.data.accessToken')"
check "管理员登录成功并拿到 accessToken" bash -c "
  [ '$login_code' = '200' ] && [ -n '$TOKEN' ] && [ '$TOKEN' != 'null' ]"

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo
  echo "登录失败，后续用例无法继续。请确认 ADMIN_PASSWORD 与后端日志中的初始密码一致。"
  echo "响应: $login_resp"
  exit 1
fi
AUTH=(-H "Authorization: Bearer $TOKEN")

# ---- 4. 追踪 ID：每个响应都应带 X-Request-ID，便于前后端日志对齐 ----
rid_headers="$(headers /api/auth/me "${AUTH[@]}")"
check "响应携带 X-Request-ID" bash -c "
  printf '%s' '$rid_headers' | grep -iq 'x-request-id:'"

# ---- 5. 身份 ----
me="$(http GET /api/auth/me '' "${AUTH[@]}")"
check "GET /api/auth/me 返回当前用户" bash -c "
  [ \"\$(printf '%s' '${me#*$TAB}' | jget '.data.user.username')\" = '$USERNAME' ]"

# ---- 6. 笔记 CRUD ----
created="$(http POST /api/notes "$(printf '{"title":"smoke-%s","body":"hello","tags":["smoke"]}' "$(date +%s)")" "${AUTH[@]}")"
note_id="$(printf '%s' "${created#*$TAB}" | jget '.data.id')"
check "创建笔记成功" bash -c "
  [ \"${created%%$TAB*}\" = '200' ] && [ -n '$note_id' ] && [ '$note_id' != 'null' ]"

notes_list="$(http GET '/api/notes' '' "${AUTH[@]}")"
check "笔记列表包含刚创建的笔记" bash -c "
  printf '%s' '${notes_list#*$TAB}' | grep -q '$note_id'"

deleted="$(http DELETE "/api/notes/$note_id" '' "${AUTH[@]}")"
check "删除笔记成功" bash -c "[ \"${deleted%%$TAB*}\" = '200' ]"

# ---- 7. 看板：白屏回归（后端严禁把空切片序列化成 null）----
board="$(http POST /api/boards '{"name":"smoke-board"}' "${AUTH[@]}")"
board_id="$(printf '%s' "${board#*$TAB}" | jget '.data.id')"
check "创建看板成功" bash -c "[ -n '$board_id' ] && [ '$board_id' != 'null' ]"

if [ -n "$board_id" ] && [ "$board_id" != "null" ]; then
  tree="$(http GET /api/boards '' "${AUTH[@]}")"
  check "看板树 JSON 不含 \"columns\":null / \"tasks\":null" bash -c "
    ! printf '%s' '${tree#*$TAB}' | grep -qE '\"(columns|tasks)\":null'"
  http DELETE "/api/boards/$board_id" '' "${AUTH[@]}" >/dev/null
fi

# ---- 7b. 入口治理：超长自由文本字段必须被拒（防超大单字段冲击下游/存储）----
long_name="$(printf 'x%.0s' $(seq 1 201))"
oversize_board="$(http POST /api/boards "{\"name\":\"$long_name\"}" "${AUTH[@]}")"
oversize_board_code="${oversize_board%%$TAB*}"
check "超长看板名称（>200 字符）被拒（400）" bash -c "[ '$oversize_board_code' = '400' ]"

# ---- 8. 网盘 ----
drive="$(http GET /api/drive '' "${AUTH[@]}")"
check "网盘文件列表可读" bash -c "[ \"${drive%%$TAB*}\" = '200' ]"

# ---- 9. 知识库：embed 服务未就绪时应降级为 503，而不是 500 ----
ingest="$(http POST /api/knowledge/ingest '{"title":"smoke","content":"冒烟测试文档内容"}' "${AUTH[@]}")"
ingest_code="${ingest%%$TAB*}"
check "知识库入库返回 200 或可重试的 503（不得为 500）" bash -c "
  [ '$ingest_code' = '200' ] || [ '$ingest_code' = '503' ]"

# ---- 10. 会话 ----
conv="$(http POST /api/agent/conversations '{"title":"smoke"}' "${AUTH[@]}")"
conv_id="$(printf '%s' "${conv#*$TAB}" | jget '.data.id')"
check "创建会话成功" bash -c "[ -n '$conv_id' ] && [ '$conv_id' != 'null' ]"
if [ -n "$conv_id" ] && [ "$conv_id" != "null" ]; then
  http DELETE "/api/agent/conversations/$conv_id" '' "${AUTH[@]}" >/dev/null
fi

# ---- 11. CORS：非白名单来源不得被回写 ----
evil="$(headers /api/auth/password-policy -H 'Origin: https://evil.example')"
check "非白名单 Origin 不回写 CORS 头" bash -c "
  ! printf '%s' '$evil' | grep -iq 'access-control-allow-origin'"

# ---- 13. 安全响应头：光有 CORS 白名单不够，浏览器侧基线防护要由应用层兜住 ----
#      （反向代理少配一段就整层失效，因此这里断言后端自身必须下发。）
sec_headers="$(headers /health)"
check "响应携带 X-Content-Type-Options: nosniff" bash -c "
  printf '%s' '$sec_headers' | grep -iq 'x-content-type-options: nosniff'"
check "响应携带 X-Frame-Options: DENY" bash -c "
  printf '%s' '$sec_headers' | grep -iq 'x-frame-options: DENY'"

# ---- 14. 请求体上限：超过额度必须返回 413，而不是「读完再用别的方式失败」 ----
#      这里打 /notes/import 而不是普通创建接口：导入是自己声明额度（10MB）的大 body 入口，
#      它必须显式判定超限并给机器可读的 413；普通 JSON 接口被绑定器吞成 400 属已知行为
#      （见 middleware.BodyLimit 注释），不作为断言目标。
#
#      注意必须走临时文件 + --data-binary @file：单个命令行参数在 Linux 上受
#      MAX_ARG_STRLEN（128KB）限制，把 11MB 直接拼进 curl -d 会让 execve 直接失败，
#      curl 静默返回空串，用例就会以一种极具误导性的方式失败。
tmp_body="$(mktemp)"
head -c 11000000 /dev/zero | tr '\0' 'x' > "$tmp_body"
oversize="$(http POST /api/notes/import '' --data-binary "@$tmp_body" "${AUTH[@]}")"
rm -f "$tmp_body"
oversize_code="${oversize%%$TAB*}"
check "超过导入额度的请求返回 413" bash -c "[ '$oversize_code' = '413' ]"

# ---- 15. 上传接口必须能抬高自己的额度（默认 8MB 覆盖不了网盘的 50MB）----
tmp_file="$(mktemp)"
printf 'starry smoke upload payload' > "$tmp_file"
upload="$(http POST /api/drive/upload '' -F "file=@$tmp_file" "${AUTH[@]}")"
rm -f "$tmp_file"
check "网盘上传走通（已抬高的 body 额度未挡住合法文件）" bash -c "
  [ \"${upload%%$TAB*}\" = '200' ]"

# ---- 16. 限流：auth:captcha 在窗口内应触发 429 ----
rate_limited=0
for _ in $(seq 1 25); do
  code="$(http POST /api/auth/captcha '' | cut -f1)"
  if [ "$code" = "429" ]; then
    rate_limited=1
    break
  fi
done
check "匿名端点触发限流（429）" bash -c "[ '$rate_limited' = '1' ]"

# ---- 17. 管理操作审计：越权冻结自己应被拒绝，且这次尝试必须落审计 ----
me_resp="$(http GET /api/auth/me '' "${AUTH[@]}")"
admin_id="$(printf '%s' "${me_resp#*$TAB}" | jget '.data.user.id')"
denied="$(http POST "/api/admin/users/$admin_id/freeze" '' "${AUTH[@]}")"
denied_code="${denied%%$TAB*}"
check "管理员冻结自己被拒绝（400）" bash -c "[ '$denied_code' = '400' ]"

# ---- 18. 审计日志可被查询，并包含上述越权尝试 ----
audit="$(http GET /api/admin/audit '' "${AUTH[@]}")"
audit_total="$(printf '%s' "${audit#*$TAB}" | jget '.data.total')"
check "审计日志接口返回 200 且有记录" bash -c "
  [ \"${audit%%$TAB*}\" = '200' ] && [ \"${audit_total:-0}\" -ge 1 ]"
check "审计记录包含越权冻结（action=user.freeze, outcome=denied）" bash -c "
  a='${audit#*$TAB}'; printf '%s' \"\$a\" | grep -q 'user.freeze' && printf '%s' \"\$a\" | grep -q 'denied'"

# ---- 19. 认证安全可观测：锁定视图端点（认证 + 管理员权限 + 结构化返回）----
lock_noauth="$(http GET /api/auth/security/lockouts)"
lock_noauth_code="${lock_noauth%%$TAB*}"
check "未带令牌访问锁定视图返回 401" bash -c "[ '$lock_noauth_code' = '401' ]"

lock="$(http GET /api/auth/security/lockouts '' "${AUTH[@]}")"
lock_code="${lock%%$TAB*}"
check "管理员访问锁定视图返回 200 且含 lockouts 数组" bash -c "
  [ '$lock_code' = '200' ] && printf '%s' '${lock#*$TAB}' | grep -q '\"lockouts\"'"

echo
echo "通过 $PASS，失败 $FAIL"
if [ "$FAIL" -ne 0 ]; then
  printf '失败用例：%s\n' "${FAILED_CASES[*]}"
  exit 1
fi
echo "冒烟通过。"

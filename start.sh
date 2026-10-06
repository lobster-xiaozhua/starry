#!/bin/bash
# Starry 本地一体化启动脚本（开发/预览用）。
# 启动顺序：PostgreSQL + Redis -> 后端(8080) -> Agent(3001) -> 前端(5173)。
# 前端通过 Vite 代理把 /api 转发到后端、/api/agent/chat 转发到 Agent。
set -e

cd "$(dirname "$0")"

# 载入本地环境变量（若存在），并补齐必要项。
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi
if [ -z "$JWT_SECRET" ]; then
  JWT_SECRET="$(openssl rand -hex 32 2>/dev/null || echo 'local-dev-insecure-secret')"
  echo "[warn] JWT_SECRET 未设置，已使用临时值（仅限本地开发）"
fi
export JWT_SECRET
export BACKEND_URL="${BACKEND_URL:-http://127.0.0.1:8080}"
export AGENT_CORS_ORIGIN="${AGENT_CORS_ORIGIN:-http://127.0.0.1:5173}"
export LLM_BASE_URL="${LLM_BASE_URL:-https://apihub.agnes-ai.com/v1}"
export LLM_MODEL="${LLM_MODEL:-agnes-3.0-flash}"

# 本地基础中间件（如已用 Docker 运行可跳过）。
service postgresql status >/dev/null 2>&1 || service postgresql start || true
redis-cli ping >/dev/null 2>&1 || redis-server --daemonize yes || true

# 后端
if [ ! -d backend/uploads ]; then mkdir -p backend/uploads; fi
( cd backend && go build -buildvcs=false -o /tmp/starry-server ./cmd/server ) \
  && echo "backend built"
UPLOAD_DIR="$PWD/backend/uploads" \
  PORT=8080 \
  DB_HOST="${DB_HOST:-127.0.0.1}" DB_PORT="${DB_PORT:-5432}" \
  DB_USER="${DB_USER:-login_user}" DB_PASSWORD="${DB_PASSWORD:-login_pass_2026}" DB_NAME="${DB_NAME:-login_system}" \
  REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}" REDIS_PASSWORD="${REDIS_PASSWORD:-}" \
  CORS_ORIGINS="${CORS_ORIGINS:-http://127.0.0.1:5173}" \
  ADMIN_USERNAME="${ADMIN_USERNAME:-admin}" ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}" \
  /tmp/starry-server &
BACKEND_PID=$!

# Agent 服务
( cd services/agent && [ -d node_modules ] || npm install --no-audit --no-fund )
( cd services/agent && PORT=3001 npx tsx src/server.ts ) &
AGENT_PID=$!

# 前端（合并后的唯一前端项目）
( cd frontend && [ -d node_modules ] || npm install --no-audit --no-fund )
trap "kill $BACKEND_PID $AGENT_PID 2>/dev/null" EXIT
( cd frontend && npm run dev )

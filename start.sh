#!/bin/bash
set -e

cd "$(dirname "$0")"

service postgresql status >/dev/null 2>&1 || service postgresql start
redis-cli ping >/dev/null 2>&1 || redis-server --daemonize yes

if [ ! -d frontend/node_modules ]; then
  (cd frontend && npm install --no-audit --no-fund)
fi

if [ ! -d apps/cloud-notes/node_modules ] && [ -d node_modules ]; then
  :
fi

(cd backend && go build -buildvcs=false -o /tmp/login-server ./cmd/server)
mkdir -p backend/uploads
UPLOAD_DIR="$PWD/backend/uploads" JWT_SECRET=notes-test-secret-fixed /tmp/login-server &
BACKEND_PID=$!

# Start AI agent service (LangGraph, port 3001)
if [ ! -d services/agent/node_modules ]; then
  (cd services/agent && npm install --no-audit --no-fund)
fi
(cd services/agent && npx tsx src/server.ts) &
AGENT_PID=$!

# Start cloud-notes PWA dev server in background
(cd apps/cloud-notes && npm run dev) &
PWA_PID=$!
trap "kill $BACKEND_PID $AGENT_PID $PWA_PID 2>/dev/null" EXIT

# Frontend login system on 5173 (primary preview port)
cd frontend && npm run dev

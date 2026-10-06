# Starry · 一体化工作台

星河工作台（Starry）是一个以**用户系统为基座**、整合 **AI 对话（日常）/ Agent 工作模式 / 云笔记** 的全栈应用。

- **用户系统**：注册、登录、验证码、刷新令牌、找回密码、管理员后台（冻结/解冻/解锁/强制改密/吊销会话）。
- **AI 对话（日常）**：轻量纯问答，基于流式 LLM。
- **Agent 工作模式**：LangGraph 编排 + 笔记工具，面向任务执行。
- **云笔记**：Markdown 笔记、标签、搜索、归档、导入导出、SSE 实时同步、PWA 离线外壳。

> 单前端项目结构：所有界面（对话 / 工作 / 笔记 / 管理）统一在 `frontend/` 中，通过左侧导航切换；后端四组 API（`auth` / `admin` / `notes` / `agent`）同时服务它们。

## 架构

```
┌───────────────┐    /api/* (nginx 反代)
│  浏览器前端    │ ───────────────────────────────┐
│ frontend:80   │                                  │
└───────────────┘                                  ▼
                                      ┌─────────────────────┐
                                      │  backend :8080 (Go) │──┐ Postgres / Redis
                                      └─────────────────────┘  │
                                               ▲                │
┌─────────────────────┐   /api/agent/chat     │                │
│ agent :3001 (Node)  │ ◀──── SSE 流式 ────────┘                │
│ LangGraph + 笔记工具 │── 对话持久化(BACKEND_URL) ──────────────┘
└─────────────────────┘
```

本地开发时，Vite 代理（`frontend/vite.config.ts`）把 `/api` → 后端 `8080`、`/api/agent/chat` → Agent `3001`。

## 目录

| 路径 | 说明 |
|---|---|
| `backend/` | Go (gin + gorm) API 服务 |
| `frontend/` | React + Vite 统一前端（登录/对话/工作/笔记/管理） |
| `services/agent/` | Node LangGraph Agent 服务 |
| `packages/shared-api/` | 前后端共享的 TS 类型与 axios 客户端 |
| `docker-compose.yml` | 一体化容器编排 |

## 快速开始

### 方式一：Docker Compose（推荐，一条命令）

```bash
cp .env.example .env          # 修改 JWT_SECRET、PUBLIC_ORIGIN、LLM_API_KEY 等
docker compose up -d --build
# 前端： http://localhost   （FRONTEND_PORT 可改）
```

首次启动后端会自动建库迁移，并在日志中打印**管理员初始密码（仅一次）**。

### 方式二：本地源码运行

需要本机 PostgreSQL、Redis、Go、Node。

```bash
./start.sh
# 前端开发服务器： http://127.0.0.1:5173
```

## 环境变量

详见 `.env.example`（docker-compose 读取）与 `backend/.env.example`（独立运行）。

| 变量 | 说明 | 默认 |
|---|---|---|
| `JWT_SECRET` | JWT 签名密钥，前后端/Agent 必须一致，**生产必填** | 留空则随机并告警 |
| `PUBLIC_ORIGIN` | 浏览器访问前端的源，用于 CORS 白名单 | `http://localhost` |
| `DB_*` | 数据库连接（容器内用 `postgres` 服务名） | — |
| `REDIS_*` | Redis 连接（容器内用 `redis` 服务名） | — |
| `LLM_API_KEY` / `LLM_BASE_URL` / `LLM_MODEL` | Agent 模型配置 | Agnes 默认 |
| `FRONTEND_PORT` | 前端容器映射端口 | `80` |
| `UPLOAD_DIR` | 附件上传目录（建议持久卷） | `uploads` |

> 安全提示：后端 CORS 已改为**白名单**模式（非白名单来源不再回写 `Access-Control-Allow-Origin`）；`JWT_SECRET` 缺失时启动会生成随机密钥并告警，生产务必显式注入稳定密钥。

## 部署要点

- **持久化**：`pgdata` / `redisdata` / `uploads` 均为命名卷，升级镜像数据不丢。
- **HTTPS**：正式环境在 nginx 前加反向代理/证书（如 Caddy、Traefik），并相应设置 `PUBLIC_ORIGIN` 为 `https://域名`。
- **密钥**：不要把真实 `.env` 提交进仓库；`.env` 已在 `.gitignore` 中。
- **Agent 运行环境**：当前 Agent 为「LLM + 笔记工具」编排，代码执行沙箱为规划项，未默认开启。

## 近期质量改进

- 合并云笔记至统一前端，单项目部署。
- 后端 CORS 白名单化；`JWT_SECRET` 缺失告警。
- 笔记导出分页累计，修复笔记数 >100 时静默截断。
- `Editor` Markdown 预览经 DOMPurify 消毒，消除 XSS。
- `/` 落地为工作台仪表盘（会话状态 + 最近笔记 + 快捷入口）。

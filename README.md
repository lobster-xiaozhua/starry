# Starry · 一体化工作台

星河工作台（Starry）是一个以**用户系统为基座**、整合 **AI 对话（日常）/ Agent 工作模式 / 云笔记** 的全栈应用。

- **用户系统**：注册、登录、验证码、刷新令牌、找回密码、管理员后台（冻结/解冻/解锁/强制改密/吊销会话）。
- **AI 对话（日常）**：轻量纯问答，基于流式 LLM。
- **Agent 工作模式（多 Agent 团队）**：`supervisor` 主调度按需委派给 `researcher`/`writer`/`coder`/`analyst` 四个子 agent，支持 agent 之间消息互联、长程多步任务。
- **企业知识库 RAG**：上传文档向量化入库（本地免费模型），`knowledge_search` 工具检索私有资料。
- **免费网络搜索**：`web_search` 直连 DuckDuckGo，无需任何密钥。
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
│ LangGraph 多Agent   │── 对话持久化(BACKEND_URL) ──────────────┘
│ + 工具集 + 知识库   │── 向量化(本地模型) / 检索(pgvector) ────┐
│ + Redis 消息总线    │                                        │
└─────────────────────┘                                        │
                                             知识库向量 ◀────────┘ (Postgres pgvector)
```

> Agent 服务在「工作模式」下使用 **supervisor + 子 agent** 编排：主调度理解目标并委派给研究员/写作/编程/分析子 agent，工具涵盖笔记、免费网络搜索、计算器、知识库检索与 agent 互联；长程任务会拆步执行并实时回写检查点。

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

首次启动后端会自动建库迁移，并在日志中打印**管理员初始密码**：

- 若在 `.env` 设置了 `ADMIN_PASSWORD`，则使用该密码；
- 否则自动生成随机密码。用 `docker compose logs backend` 查看（仅首次建库时打印一次）。

Compose 内置健康检查（后端 `/health` 探活 PostgreSQL+Redis、Agent `/health`），前端会等待后端与 Agent **健康后**再启动，避免冷启动白屏。登录后若数据为空，可在工作台点「一键加载示例」或笔记页「加载示例笔记」快速体验（后端幂等，不会重复灌入）。

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
| `ADMIN_USERNAME` / `ADMIN_EMAIL` | 初始管理员账号 | `admin` / `admin@example.com` |
| `ADMIN_PASSWORD` | 初始管理员密码（留空则随机生成并打印日志；`APP_ENV=production` 时留空会拒绝启动） | 留空 |
| `APP_ENV` | `production` 启用配置 fail-fast 校验与 JSON 日志 | `production`（compose）/ `development` |
| `LOG_LEVEL` | 日志级别，留空按环境推导 | 留空 |
| `CAPTCHA_ENABLED` | 首次建库时写入的验证码开关，之后由管理端设置接管 | `true` |
| `AGENT_INTERNAL_TOKEN` | 后端↔Agent 内部调用共享令牌（知识库向量化等内部端点鉴权） | `starry-internal` |
| `FRONTEND_PORT` | 前端容器映射端口 | `80` |
| `UPLOAD_DIR` | 附件上传目录（建议持久卷） | `uploads` |

> 安全提示：后端 CORS 已改为**白名单**模式（非白名单来源不再回写 `Access-Control-Allow-Origin`）；`JWT_SECRET` 缺失时启动会生成随机密钥并告警，生产务必显式注入稳定密钥。

## AI Agent 平台（v2）

Starry 的 Agent 已从「单一笔记工具助手」升级为**多 Agent 协作平台**：

- **主调度 supervisor**：理解用户目标，拆解为子任务，通过 `delegate_to_<角色>` 工具委派给子 agent，并综合结果。
- **四个常用子 agent**（均可在 `services/agent/src/config/agents.config.ts` 自由增删）：
  - `researcher` 研究员：联网调研（web_search / web_fetch）+ 知识库检索。
  - `writer` 写作助手：起草/总结，并沉淀到云笔记（create_note）。
  - `coder` 编程助手：技术方案、代码片段、文档查阅。
  - `analyst` 分析师：数据归纳与量化分析（calculator）。
- **工具集**：笔记工具、免费 `web_search`（DuckDuckGo，零密钥）、`web_fetch`、`calculator`、`current_time`、`knowledge_search`、agent 互联（`message_agent` / `read_messages`）。
- **Agent 消息互联**：基于 Redis 信箱 + 发布订阅，agent 之间可异步投递与收取消息。
- **长程任务**：`POST /api/agent/tasks` 提交高层目标，Agent 自动拆步、逐步执行、每步检查点回写后端（`agent_tasks` 表），并通过 `GET /api/agent/tasks/:id/stream` 以 SSE 实时推送进度，支持取消与步数上限（默认 12 步）。
- **企业知识库 RAG（B 方案）**：`POST /api/knowledge/ingest` 上传文本，由**独立的 embed 向量化服务**用**本地免费模型**（Xenova/all-MiniLM-L6-v2，约 25MB，首次自动下载）向量化，存入 Postgres `pgvector`；`GET /api/knowledge/search` 做余弦相似检索，供 `knowledge_search` 工具使用。向量化已与对话 / Agent 服务解耦，嵌入模型下载被网络拦截时只影响知识库，不会拖垮对话。

### 开箱使用示例配置

子 agent 团队与工具白名单即「开箱可用示例」，位于 `services/agent/src/config/agents.config.ts`。
修改该文件即可增删子 agent、调整 system prompt 与工具集，**无需改动编排代码**。重启 Agent 服务生效。

### 关键端点

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/knowledge/ingest` | 知识库入库（文本 → 向量） |
| GET | `/api/knowledge/search?q=` | 知识库相似检索 |
| GET | `/api/knowledge/docs` | 文档列表 |
| POST | `/api/agent/tasks` | 创建并运行长程任务 |
| GET | `/api/agent/tasks/:id/stream` | 任务进度 SSE |
| POST | `/api/agent/embed` | 内部向量化（embed 服务，需 `X-Internal-Token`） |

## 部署要点

- **持久化**：`pgdata` / `redisdata` / `uploads` 均为命名卷，升级镜像数据不丢。Postgres 使用 `pgvector/pgvector` 镜像以支持知识库向量检索。
- **HTTPS**：正式环境在 nginx 前加反向代理/证书（如 Caddy、Traefik），并相应设置 `PUBLIC_ORIGIN` 为 `https://域名`。
- **密钥**：不要把真实 `.env` 提交进仓库；`.env` 已在 `.gitignore` 中。生产请为 `JWT_SECRET` 与 `AGENT_INTERNAL_TOKEN` 设置强随机值。
- **Agent 运行环境**：Agent 平台已实现多 agent 协作、长程任务与知识库 RAG。代码执行沙箱（直接运行用户代码）为独立安全项，默认未开启——如需「运行环境」能力，建议接入 gVisor/函数计算等隔离方案。
- **出网收敛**：所有对外请求只走一处受管控的出口（Go：`embed_client.go`；Node：`services/agent/src/http.ts`），统一带超时、体积上限与重试策略。URL 来自 LLM 或用户时（`web_fetch`）额外启用 SSRF 防护：拦截内网/链路本地/云元数据地址，DNS 预解析校验，重定向逐跳复检。
- **请求入口治理**：全局请求体上限 `MAX_BODY_BYTES`（默认 8 MiB）保护常规 JSON 接口；上传/导入这类真正接收大 body 的入口用 `RaiseBodyLimit` 显式声明自己的额度（超限返回 413），杜绝「网盘能传 50MB、笔记正文却被同一个限额卡住」。应用自己下发 `nosniff` / `X-Frame-Options` / `Referrer-Policy` 等基线安全头，不把防线全押在反向代理配置上。
- **查询参数治理与审计**：列表接口的 `page` / `size` / `limit` / `k` 等参数统一经 `internal/request` 包钳制（缺省/非法回落确定值，数量参数带硬上限），`order by` 走白名单杜绝 SQL 注入。管理端的冻结/解冻/解锁/强制改密/吊销会话/修改安全设置等操作全部写入审计日志（`/api/admin/audit`），记录操作人、目标、结果（success/failed/denied）与来源 IP/追踪码——JWT 新增 `username` 声明使审计条目自带可读身份。

## 备份与恢复

```bash
npm run backup                                  # 输出到 ./backups（可用 BACKUP_DIR / KEEP 调整）
echo <库名> | npm run restore backups/db-<时间戳>.sql.gz
```

备份包含三件：数据库（`pg_dump`）、笔记附件卷 `uploads`、网盘卷 `drive`，三份共用同一时间戳。
恢复会**清空 `public` schema 后重建**，因此需要手动输入库名二次确认。

## 验证与门禁

本地一键跑通全部静态检查：

```bash
npm run check:modules      # 架构守卫：业务模块不得横向依赖
npm run typecheck:services # agent / embed 服务类型检查
npm run test:services      # Node 服务测试（node --test，无需测试框架）
npm run check:compose      # compose 配置校验
cd backend && go test ./... && go vet ./...
```

跑起来之后再做一次端到端冒烟（覆盖登录、笔记、看板、网盘、知识库降级、CORS、限流等跨模块契约）：

```bash
docker compose -f docker-compose.yml -f docker-compose.smoke.yml up -d --build postgres redis embed backend
ADMIN_PASSWORD=<你的密码> npm run smoke
```

## 近期质量改进

- 合并云笔记至统一前端，单项目部署。
- 后端 CORS 白名单化；`JWT_SECRET` 缺失告警。
- 笔记导出分页累计，修复笔记数 >100 时静默截断。
- `Editor` Markdown 预览经 DOMPurify 消毒，消除 XSS。
- `/` 落地为工作台仪表盘（会话状态 + 最近笔记 + 快捷入口）。
- 一条龙开箱：后端 `/health` + Agent `/health` 健康检查，Compose 按健康顺序启动。
- 初始管理员密码可通过 `ADMIN_PASSWORD` 指定（留空则随机打印）。
- 空数据一键体验：工作台「一键加载示例」/ 笔记页「加载示例笔记」（幂等）。
- 多 Agent 协作平台（supervisor + 4 个子 agent）、长程任务、知识库 RAG 与免费网络搜索。
- 新增「知识库」界面（`/knowledge`）：文本入库、相似检索、文档管理；并含长程任务提交与实时进度。
- 长程任务端点（`POST /api/agent/tasks`、`/stream`）在 nginx 与 Vite 中正确代理至 Agent 服务。
- 后端首组单元测试（知识库分块与向量序列化，表驱动）。
- 前端按变更频率拆分 vendor 包，应用主包 1160 kB → 159 kB。
- 后端改为模块化单体：8 个业务模块各自持有服务与迁移，可用 `MODULES_<NAME>_ENABLED=false` 单独停用；CI 用脚本守卫「模块之间不得横向依赖」。
- 统一结构化日志（`slog`，生产输出 JSON）与全链路 `X-Request-ID`；访问日志对 query 中的 token/password 等凭据脱敏。
- 限流计数下沉到 Redis，多副本部署时额度不再随实例数放大；计数不可用时限流 fail-open，由登录失败锁定兜底。
- 两个 Node 服务纳入类型检查与 `node --test` 单元测试；新增 `scripts/smoke.sh` 端到端冒烟。
- 出网 HTTP 韧性：Go 侧知识库向量化改为共享连接池 + 分批 + 有界重试 + 返回条数契约校验（不再每次请求建池）；Node 侧新增统一出口 `http.ts`，为 `web_fetch` 这类「URL 由 LLM 决定」的调用补上 SSRF 防护、超时与响应体上限。
- 请求入口治理：新增 `middleware.BodyLimit` / `RaiseBodyLimit` / `IsBodyTooLarge` 与安全响应头中间件；`core.Deps.Limiter` 改为接口，解开 `core ↔ middleware` 的循环依赖。笔记导入不再静默截断超限内容（改为明确 413）。
- 配置损坏防线：`frontend/Dockerfile` 与 CI 曾用 `sed` 改写 `package.json`，留下尾随逗号、构建阶段才炸；改为 Node 重写，并新增 `scripts/check-json.js` 让这类问题秒级暴露。
- 列表查询与排序治理：新增 `internal/request` 包（分页 clamp + 数量上限 + order-by 白名单），笔记/对话/知识检索等读接口统一收敛；管理端补齐操作审计日志，JWT 增加 `username` 声明。

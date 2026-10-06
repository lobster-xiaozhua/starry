# Starry 夜间迭代日志

- 分支：`night-20261007`（自 `main` 创建，起始提交 `55bf24a`）
- 构建：`go build ./...` / `go vet ./...` / `gofmt -l`（后端）、`npm run build`（前端，含 tsc）、`npx tsc --noEmit`（agent）
- 测试：`go test ./...`（后端）、`npx tsc --noEmit` + 冒烟（agent）
- 安全约束：不 push、不修改 `.env`/密钥/生产配置/数据库迁移

## 迭代 1 — 落库上一轮多 Agent + 知识库实现
- 类型：P0/P1（未落库实现先固化） | 提交 `e1c5ec2`
- 29 文件：多 agent 编排、工具集、长程任务、知识库 RAG、pgvector 编排
- 验证：go build ✅ · go vet ✅ · gofmt ✅ · agent tsc ✅ · frontend build ✅ · compose config ✅

## 迭代 2 — 后端首组单元测试
- 类型：P1（仓库此前零测试文件） | 提交 `a1994bb`
- `TestChunkText` / `TestChunkTextCoversWholeText` / `TestVectorLiteral` / `TestVectorLiteralIsParseable`
- 验证：`go test ./...` ✅ · gofmt ✅

## 迭代 3 — 修复长程任务端点不可达（关键集成缺陷）
- 类型：P0（功能实际不可达） | 提交 `dd44699`
- 发现：nginx/Vite 仅把 `/api/agent/chat` 精确转发到 3001，其余 `/api/agent/*` 全走后端；
  而「创建并运行」与 SSE 进度流在 agent 服务上 → 前端无法触发执行
- 修复：nginx 新增 `location /api/agent/tasks`（关缓冲以支持 SSE）；
  Vite 新增 `/api/agent/tasks` 代理；agent 服务补 `POST /tasks/:id/cancel` 转发后端
- 验证：agent tsc ✅ · frontend build ✅ · compose config ✅

## 迭代 4 — 前端 API 客户端
- 类型：P1 | 提交 `4213669`
- `api/knowledge.ts`、`api/tasks.ts`
- 决策：任务进度订阅用 fetch + ReadableStream（与既有 streamChat 一致），
  刻意不用 EventSource——它无法带 Authorization 头，JWT 进 URL 会泄漏到日志
- 验证：frontend tsc ✅

## 迭代 5 — 知识库 / 长程任务界面
- 类型：P1 | 提交 `b8563c7`
- `pages/Knowledge.tsx`（入库/检索/文档管理 + 任务提交/实时进度/历史）、
  `/knowledge` 路由、导航「知识库」入口
- 验证：frontend build（含 tsc）✅

## 迭代 6 — 打包体积优化
- 类型：P2 | 提交 `405080e`
- 拆分为 react-vendor / antd-vendor / editor-vendor；主包 1160 kB → 158.79 kB（-86%）
- 遗留：antd 仍 910 kB 并触发告警（体积固有、已独立缓存），按需引入属较大重构

## 停止原因
达到计划迭代上限（6 次有效迭代），且剩余项需人工决策（见报告「建议下一步」）。

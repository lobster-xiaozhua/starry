# Starry 夜间迭代日志

- 分支：`night-20261007`（自 `main` 创建）
- 起始提交：`55bf24a`
- 构建命令：`go build ./...` / `go vet ./...` / `gofmt -l`（后端）、`npm run build`（前端，含 tsc）、`npx tsc --noEmit`（agent）
- 测试命令：`go test ./...`（后端）、`npx tsc --noEmit` + 冒烟（agent）
- 安全约束：不 push、不修改 `.env`/密钥/生产配置/数据库迁移

## 迭代 1 — 落库上一轮多 Agent + 知识库实现

- **类型**：P0/P1（未落库的既有实现需先固化）
- **改动**：29 文件（多 agent 编排、工具集、长程任务、知识库 RAG、pgvector 编排）
- **验证**：`go build` ✅ · `go vet` ✅ · `gofmt`（新文件）✅ · agent `tsc` ✅ · frontend `build` ✅ · `compose config` ✅
- **提交**：`e1c5ec2`
- **备注**：工作区已清空，无 `.env` 残留

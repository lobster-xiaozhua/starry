# Repository Guidelines

## Project Structure & Module Organization

This is a small full-stack workspace:

- `backend/` contains the Go API server, organized as a **modular monolith**. `cmd/server/main.go` is a composition root: it builds shared infrastructure and a `core.Deps` container, then calls `modules.RegisterAll` — business routes do not appear in `main`. Each domain lives in `internal/modules/<name>/` (`auth`, `admin`, `knowledge`, `notes`, `chat`, `boards`, `drive`, `vault`), owns its handler and route registration (`Register`), and may depend only on infrastructure (`core`, `middleware`, `model`, `store`, `config`, `service`, `sse`) — **never on another module**. Any module can be disabled without touching code via `MODULES_<NAME>_ENABLED=false`.
- `frontend/` is the primary React/Vite login and administration UI, served on port `5173`.
- `apps/cloud-notes/` is a second React/Vite notes PWA, served on port `5174`.
- `packages/shared-api/` contains TypeScript API types/client code shared by both frontends.
- `services/agent/` is the conversation / agent-orchestration service (LangGraph + LLM streaming, SSE). It no longer performs embeddings.
- `services/embed/` is a standalone vectorization (embedding) service built on `@xenova/transformers`; it is decoupled from chat so that embedding failures (e.g. model download blocked) never break the conversation flow. The Go backend calls it via `EMBED_URL`.
- `backend/.env.example` documents backend configuration. Static assets belong in each app's `public/` directory.

## Build, Test, and Development Commands

Run `npm install` at the repository root to install workspace dependencies. Use `./start.sh` for the integrated local environment; it starts PostgreSQL and Redis, builds the Go server, and launches both Vite apps. For focused work, run `npm run dev` in `frontend/` or `apps/cloud-notes/`, and use `go run ./cmd/server` from `backend/`.

Build the frontends with `npm run build` in either frontend package. Build the API with `go build ./cmd/server` from `backend/`. Run Go checks with `go test ./...`. The repository ships focused unit tests (e.g. `internal/config`, `internal/authpkg`); add regression coverage alongside the affected package when changing backend behavior.

## Coding Style & Naming Conventions

Use Go formatting via `gofmt` and standard Go naming (exported identifiers in `PascalCase`, local variables in `camelCase`). Keep backend packages organized by responsibility. Use TypeScript/React with two-space indentation, `PascalCase` component names, `camelCase` functions and hooks, and `*.tsx` for JSX-bearing files. Keep shared contracts in `packages/shared-api` rather than duplicating them in an app.

## Testing Guidelines

Prefer table-driven Go tests named after the behavior under test, such as `TestValidatePassword`. For frontend changes, run the relevant `npm run build` because TypeScript compilation is part of that command. Add regression coverage alongside the affected package when practical.

## Commit & Pull Request Guidelines

No usable Git history is present in this checkout, so use concise imperative commit subjects (for example, `Fix note search pagination`). Keep commits focused. Pull requests should explain the user-visible or API change, identify configuration or migration needs, list validation commands, and include screenshots for UI changes. Never commit secrets; use `backend/.env.example` as the configuration template.

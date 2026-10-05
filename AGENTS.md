# Repository Guidelines

## Project Structure & Module Organization

This is a small full-stack workspace:

- `backend/` contains the Go API server. The entry point is `cmd/server/main.go`; handlers, services, stores, models, middleware, and SSE support live under `internal/`.
- `frontend/` is the primary React/Vite login and administration UI, served on port `5173`.
- `apps/cloud-notes/` is a second React/Vite notes PWA, served on port `5174`.
- `packages/shared-api/` contains TypeScript API types/client code shared by both frontends.
- `backend/.env.example` documents backend configuration. Static assets belong in each app's `public/` directory.

## Build, Test, and Development Commands

Run `npm install` at the repository root to install workspace dependencies. Use `./start.sh` for the integrated local environment; it starts PostgreSQL and Redis, builds the Go server, and launches both Vite apps. For focused work, run `npm run dev` in `frontend/` or `apps/cloud-notes/`, and use `go run ./cmd/server` from `backend/`.

Build the frontends with `npm run build` in either frontend package. Build the API with `go build ./cmd/server` from `backend/`. Run Go checks with `go test ./...`; currently the repository has no committed test files, so add focused tests when changing backend behavior.

## Coding Style & Naming Conventions

Use Go formatting via `gofmt` and standard Go naming (exported identifiers in `PascalCase`, local variables in `camelCase`). Keep backend packages organized by responsibility. Use TypeScript/React with two-space indentation, `PascalCase` component names, `camelCase` functions and hooks, and `*.tsx` for JSX-bearing files. Keep shared contracts in `packages/shared-api` rather than duplicating them in an app.

## Testing Guidelines

Prefer table-driven Go tests named after the behavior under test, such as `TestValidatePassword`. For frontend changes, run the relevant `npm run build` because TypeScript compilation is part of that command. Add regression coverage alongside the affected package when practical.

## Commit & Pull Request Guidelines

No usable Git history is present in this checkout, so use concise imperative commit subjects (for example, `Fix note search pagination`). Keep commits focused. Pull requests should explain the user-visible or API change, identify configuration or migration needs, list validation commands, and include screenshots for UI changes. Never commit secrets; use `backend/.env.example` as the configuration template.

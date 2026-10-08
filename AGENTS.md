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
- Cross-cutting infrastructure lives outside modules and is wired once by the composition root: `internal/logx` (structured `slog` logging, JSON in production), `internal/middleware` (request ID, access log with credential redaction, CORS, JWT, and a `Limiter` whose counter is Redis-backed so quotas are shared across replicas), and `internal/store` (Postgres/Redis/file/drive stores).

## Observability & Rate Limiting

- **Logging**: never use the standard `log` package in `backend/`. Use `logx.L()` (or the package-level `slog`), initialized once by `logx.Setup(env, level)` in `main`. Production emits JSON to stdout; development emits text to stderr at debug level.
- **Request tracing**: `middleware.RequestID()` must run first in the chain; access logs (`middleware.RequestLogger`) carry `request_id`, status, latency and a redacted path. Any new query parameter that carries a credential must be added to `sanitizePath`.
- **Rate limiting**: endpoints that can be abused anonymously take `d.Limiter.Limit("<scope>", max, window)` from `core.Deps`. Do not call `middleware.RateLimit` in a module — its in-memory counter silently multiplies the quota by the replica count. The limiter fails open when the counter is unavailable, so brute-force defense still depends on the login lockout counters.
- **Dependency direction**: infrastructure may import `core` (it reuses the unified response envelope), therefore `core` must **not** import any infrastructure package. `core.RateLimiter` exists as an interface for exactly this reason — `middleware.Limiter` satisfies it, and `core` never learns it exists.

## Request Ingress

- **Body size**: every request is bounded by `middleware.BodyLimit(cfg.MaxBodyBytes)` installed once in the composition root. An endpoint that legitimately accepts more (uploads, imports) calls `middleware.RaiseBodyLimit(c, n)` at the top of its handler with a comment naming why. Never wrap `http.MaxBytesReader` inside a handler — it stacks on top of the global limit instead of replacing it.
- **Too-large detection**: use `middleware.IsBodyTooLarge(err)` (`errors.As` against `*http.MaxBytesError`), never a substring match on the error text. Endpoints whose contract is "accept large input" return 413 themselves; ordinary JSON handlers letting `ShouldBindJSON` surface the failure will report 400 — that is a known trade-off documented in `middleware/body.go`, not a bug to patch by special-casing.
- **Response headers**: `middleware.SecurityHeaders` sets nosniff/frame-deny/referrer/permissions; HSTS is opt-in per environment. Adding a new credential-bearing query parameter also means adding it to `middleware.sanitizePath` so access logs stay redacted.

## Outbound HTTP

Every request this system makes to *another* service must go through one shared, hardened client — never an ad-hoc `http.Client` or bare `fetch`:

- **Go backend**: the only outbound call today is `internal/modules/knowledge/embed_client.go` (`embedClient`). It owns a pooled `http.Transport` built once at startup, binds every request to the caller's context, splits oversized payloads into batches, retries only retryable failures (network errors, 429, 5xx) with backoff, and validates the response contract (vector count must equal input count). New outbound calls must follow the same shape.
- **Node services**: all external requests go through `services/agent/src/http.ts`. Use `fetchGuarded()` for **any URL that is not a hardcoded internal endpoint** — especially URLs produced by the LLM — because it enforces SSRF protection (blocks private/link-local/metadata addresses, pre-resolves DNS, re-validates every redirect hop) plus a hard timeout and a streaming body size cap. Use `fetchWithTimeout()` for calls to our own backend defined by config.

The rule of thumb: a request whose target comes from user or model input is untrusted, and must not be able to reach `169.254.169.254`, RFC1918 space, or loopback.

## Build, Test, and Development Commands

Run `npm install` at the repository root to install workspace dependencies. Use `./start.sh` for the integrated local environment; it starts PostgreSQL and Redis, builds the Go server, and launches both Vite apps. For focused work, run `npm run dev` in `frontend/` or `apps/cloud-notes/`, and use `go run ./cmd/server` from `backend/`.

Build the frontends with `npm run build` in either frontend package. Build the API with `go build ./cmd/server` from `backend/`. Run Go checks with `go test ./...`. The repository ships focused unit tests (e.g. `internal/config`, `internal/authpkg`); add regression coverage alongside the affected package when changing backend behavior.

From the repository root, the whole quality gate is: `npm run check:modules` (fails if a business module imports another module or a non-infrastructure package), `npm run typecheck:services`, `npm run test:services` (Node service tests run with `node --import tsx --test`, no test framework dependency), and `npm run check:compose`. CI runs all of these plus `gofmt`/`go vet`/`go test -race` and the frontend build.

## Coding Style & Naming Conventions

Use Go formatting via `gofmt` and standard Go naming (exported identifiers in `PascalCase`, local variables in `camelCase`). Keep backend packages organized by responsibility. Use TypeScript/React with two-space indentation, `PascalCase` component names, `camelCase` functions and hooks, and `*.tsx` for JSX-bearing files. Keep shared contracts in `packages/shared-api` rather than duplicating them in an app.

## Testing Guidelines

Prefer table-driven Go tests named after the behavior under test, such as `TestValidatePassword`. For frontend changes, run the relevant `npm run build` because TypeScript compilation is part of that command. Add regression coverage alongside the affected package when practical.

## Commit & Pull Request Guidelines

No usable Git history is present in this checkout, so use concise imperative commit subjects (for example, `Fix note search pagination`). Keep commits focused. Pull requests should explain the user-visible or API change, identify configuration or migration needs, list validation commands, and include screenshots for UI changes. Never commit secrets; use `backend/.env.example` as the configuration template.

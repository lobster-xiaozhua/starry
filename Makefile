SHELL := /bin/bash
BACKEND_DIR := backend
FRONTEND_DIR := frontend
AGENT_DIR := services/agent
EMBED_DIR := services/embed

.PHONY: dev build test lint fmt check docker-up docker-down smoke backup restore clean install-hooks docs migrate-up migrate-down

## dev: start local dev stack (calls start.sh)
dev:
	./start.sh

## build: Go binary + frontend + Node service tsc
build:
	cd $(BACKEND_DIR) && go build -o /tmp/starry-server ./cmd/server
	cd $(FRONTEND_DIR) && npm run build
	cd $(AGENT_DIR) && npm run build
	cd $(EMBED_DIR) && npm run build

## test: Go tests (with -race) + Node service tests + frontend tsc
test:
	cd $(BACKEND_DIR) && go test -race -count=1 ./...
	npm run test:services --if-present
	cd $(FRONTEND_DIR) && npx tsc --noEmit

## lint: golangci-lint + ESLint
lint:
	cd $(BACKEND_DIR) && golangci-lint run ./...
	npm run lint --workspaces --if-present || true

## fmt: gofmt + prettier
fmt:
	cd $(BACKEND_DIR) && gofmt -w .
	npx prettier --write "frontend/src/**/*.{ts,tsx}" "services/*/src/**/*.ts" || true

## check: CI-equivalent gate (lint + test + module isolation + json + compose)
check: lint test
	npm run check:modules
	npm run check:json
	npm run check:compose

## docker-up: build and start full stack
docker-up:
	docker compose up -d --build

## docker-down: stop full stack
docker-down:
	docker compose down

## smoke: end-to-end smoke test
smoke:
	ADMIN_PASSWORD=$$(grep ADMIN_PASSWORD .env 2>/dev/null | cut -d= -f2 || echo admin) bash scripts/smoke.sh

## backup: database backup
backup:
	bash scripts/backup.sh

## restore: database restore
restore:
	bash scripts/restore.sh

## clean: remove build artifacts
clean:
	rm -rf /tmp/starry-server
	rm -rf $(FRONTEND_DIR)/dist
	rm -rf $(AGENT_DIR)/dist
	rm -rf $(EMBED_DIR)/dist
	find . -name "*.tsbuildinfo" -delete 2>/dev/null || true

## install-hooks: install pre-commit git hooks
install-hooks:
	pre-commit install

## docs: generate OpenAPI docs (swaggo)
docs:
	cd $(BACKEND_DIR) && swag init -g cmd/server/main.go -o docs

## migrate-up: run database migrations forward
migrate-up:
	cd $(BACKEND_DIR) && go run ./cmd/server --migrate up

## migrate-down: roll back one database migration
migrate-down:
	cd $(BACKEND_DIR) && go run ./cmd/server --migrate down

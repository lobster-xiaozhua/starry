#!/bin/bash
## Print current migration version and pending migrations.
## Usage: bash backend/scripts/migrate-status.sh
set -euo pipefail

cd "$(dirname "$0")/.."

DSN="postgres://${DB_USER:-login_user}:${DB_PASSWORD:-login_pass_2026}@${DB_HOST:-127.0.0.1}:${DB_PORT:-5432}/${DB_NAME:-login_system}?sslmode=${DB_SSLMODE:-disable}"

echo "=== Migration Status ==="
go run ./cmd/server --migrate version
echo ""
echo "=== Applied Migrations ==="
PGPASSWORD="${DB_PASSWORD:-login_pass_2026}" psql -h "${DB_HOST:-127.0.0.1}" -p "${DB_PORT:-5432}" -U "${DB_USER:-login_user}" -d "${DB_NAME:-login_system}" -c "SELECT version, dirty, applied_at FROM schema_migrations ORDER BY version DESC LIMIT 10;" 2>/dev/null || echo "(schema_migrations table not found - using AutoMigrate fallback)"
echo ""
echo "=== Migration Files ==="
ls -1 migrations/*.sql 2>/dev/null || echo "(no migration files found)"

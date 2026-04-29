#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
CORE_DB_URL="postgres://postgres:postgres@localhost:5433/core_db?sslmode=disable"

cd "$REPO_ROOT"

# ── 1. Инфраструктура ────────────────────────────────────────────────────────
echo "▶ Запуск инфраструктуры..."
docker compose -f deploy/docker-compose.dev.yml up -d core-db redis zookeeper kafka kafka-init

echo "▶ Ожидание core-db..."
until docker compose -f deploy/docker-compose.dev.yml exec -T core-db \
    pg_isready -U postgres -d core_db -q 2>/dev/null; do
  sleep 1
done
echo "  core-db готова"

# ── 2. Миграции ──────────────────────────────────────────────────────────────
echo "▶ Применение миграций..."
atlas migrate apply \
  --dir "file://services/core/migrations" \
  --url "$CORE_DB_URL"

# ── 3. Swagger ───────────────────────────────────────────────────────────────
echo "▶ Генерация Swagger..."
cd services/core
swag init -g cmd/api/swagger.go -o docs/swagger --parseDependency -q
cd "$REPO_ROOT"

# ── 4. Запуск сервиса ────────────────────────────────────────────────────────
echo ""
echo "✓ Swagger UI: http://localhost:8081/swagger/index.html"
echo ""
cd services/core
go run ./cmd/api

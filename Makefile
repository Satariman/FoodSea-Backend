.PHONY: all build generate proto swagger swagger-optimization test test-unit test-integration test-cover test-optimization test-e2e-core test-e2e-ordering test-e2e-optimization test-go-all test-swagger-regression test-ml test-all lint \
        docker-build dev-infra-up dev-infra-down dev-core dev-ordering dev-all stop-all \
        k8s-render k8s-deploy k8s-down k8s-logs k8s-status \
        migrate-core migrate-optimization migrate-ordering migrate \
        atlas-diff-core atlas-diff-ordering atlas-hash \
        seed-core \
        tools

# ── Build ────────────────────────────────────────────────────────────────────

build:
	cd services/core && go build -o ../../bin/core ./cmd/api
	cd services/optimization && go build -o ../../bin/optimization ./cmd/api
	cd services/ordering && go build -o ../../bin/ordering ./cmd/api

# ── Code generation ──────────────────────────────────────────────────────────

generate:
	cd services/core && go generate ./ent
	cd services/optimization && go generate ./ent
	cd services/ordering && go generate ./ent

proto:
	protoc --go_out=proto --go_opt=paths=source_relative \
	       --go-grpc_out=proto --go-grpc_opt=paths=source_relative \
	       -I proto \
	       proto/core/cart.proto \
	       proto/core/catalog.proto \
	       proto/core/offers.proto \
	       proto/ml/analogs.proto \
	       proto/optimization/optimization.proto

swagger:
	cd services/core && swag init -g cmd/api/swagger.go -o docs/swagger --parseDependency -q
	cd services/optimization && swag init -g cmd/api/main.go -o docs/swagger --parseDependency -q
	cd services/ordering && swag init -g cmd/api/swagger.go -o docs/swagger --parseDependency -q

swagger-optimization:
	cd services/optimization && swag init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal

# ── Tests ────────────────────────────────────────────────────────────────────

test-unit:
	cd services/core && go test ./internal/... -short -count=1
	cd services/optimization && go test ./internal/... -short -count=1
	cd services/ordering && go test ./internal/... -short -count=1

test-integration:
	cd services/core && go test -tags integration ./internal/... -count=1
	cd services/optimization && go test -tags integration ./internal/... -count=1
	cd services/ordering && go test -tags integration ./internal/... -count=1

test-cover:
	cd services/core && go test -coverprofile=coverage.out ./internal/... && \
		go tool cover -func=coverage.out
	cd services/optimization && go test -coverprofile=coverage.out ./internal/... && \
		go tool cover -func=coverage.out
	cd services/ordering && go test -coverprofile=coverage.out ./internal/... && \
		go tool cover -func=coverage.out

test-e2e-core:
	cd services/core && go test -tags e2e -count=1 -timeout 5m ./test/e2e/...

test-e2e-ordering:
	cd services/ordering && go test -tags e2e -count=1 -timeout 5m ./test/e2e/...

test-optimization:
	cd services/optimization && go test ./... -short -count=1

test-e2e-optimization:
	cd services/optimization && go test -tags e2e ./test/e2e/ -v -count=1 -timeout 120s

test-go-all: test-unit test-integration test-e2e-core test-e2e-ordering test-e2e-optimization

SWAGGER_CORE_BASE_URL ?= http://localhost:8081
SWAGGER_OPTIMIZATION_BASE_URL ?= http://localhost:8082
SWAGGER_ORDERING_BASE_URL ?= http://localhost:8083
SWAGGER_REGRESSION_TIMEOUT ?= 15
SWAGGER_REGRESSION_PASSWORD ?= Passw0rd!123
SWAGGER_REGRESSION_REPORT_PREFIX ?= reports/swagger-regression-$(shell date +%Y%m%d-%H%M%S)

test-swagger-regression:
	@set -e; \
	core_up=0; opt_up=0; ord_up=0; \
	curl -sf "$(SWAGGER_CORE_BASE_URL)/health" >/dev/null && core_up=1 || true; \
	curl -sf "$(SWAGGER_OPTIMIZATION_BASE_URL)/health" >/dev/null && opt_up=1 || true; \
	curl -sf "$(SWAGGER_ORDERING_BASE_URL)/health" >/dev/null && ord_up=1 || true; \
	owned_stack=0; \
	if [ "$$core_up" -eq 1 ] && [ "$$opt_up" -eq 1 ] && [ "$$ord_up" -eq 1 ]; then \
		echo "Using already running services for swagger regression."; \
	else \
		if [ "$$core_up" -eq 0 ] && [ "$$opt_up" -eq 0 ] && [ "$$ord_up" -eq 0 ]; then \
			owned_stack=1; \
		else \
			echo "Detected partially running stack; starting missing services and keeping them running after test."; \
		fi; \
		echo "Starting services via make dev-all..."; \
		$(MAKE) dev-all; \
	fi; \
	cleanup() { \
		status=$$?; \
		if [ "$$owned_stack" -eq 1 ]; then \
			echo "Stopping services started by test-swagger-regression..."; \
			$(MAKE) stop-all; \
		fi; \
		exit $$status; \
	}; \
	trap cleanup EXIT INT TERM; \
	wait_health() { \
		url="$$1"; \
		name="$$2"; \
		attempts=45; \
		while [ "$$attempts" -gt 0 ]; do \
			if curl -sf "$$url/health" >/dev/null; then \
				return 0; \
			fi; \
			attempts=$$((attempts - 1)); \
			sleep 2; \
		done; \
		echo "$$name is not reachable at $$url"; \
		return 1; \
	}; \
	wait_health "$(SWAGGER_CORE_BASE_URL)" "core-service"; \
	wait_health "$(SWAGGER_OPTIMIZATION_BASE_URL)" "optimization-service"; \
	wait_health "$(SWAGGER_ORDERING_BASE_URL)" "ordering-service"; \
	python3 scripts/swagger_regression_runner.py \
		--core-base-url "$(SWAGGER_CORE_BASE_URL)" \
		--optimization-base-url "$(SWAGGER_OPTIMIZATION_BASE_URL)" \
		--ordering-base-url "$(SWAGGER_ORDERING_BASE_URL)" \
		--password "$(SWAGGER_REGRESSION_PASSWORD)" \
		--timeout "$(SWAGGER_REGRESSION_TIMEOUT)" \
		--report-prefix "$(SWAGGER_REGRESSION_REPORT_PREFIX)"

test-ml:
	cd services/ml && $(MAKE) test

test-all: test-go-all test-ml test-swagger-regression

test: test-unit

# ── Lint ─────────────────────────────────────────────────────────────────────

lint:
	cd services/core && golangci-lint run ./...
	cd services/optimization && golangci-lint run ./...
	cd services/ordering && golangci-lint run ./...

# ── Docker ───────────────────────────────────────────────────────────────────

IMAGE_REGISTRY ?= foodsea
IMAGE_TAG      ?= latest

docker-build:
	docker build -f services/core/Dockerfile -t $(IMAGE_REGISTRY)/core-service:$(IMAGE_TAG) .
	docker build -f services/optimization/Dockerfile -t $(IMAGE_REGISTRY)/optimization-service:$(IMAGE_TAG) .
	docker build -f services/ordering/Dockerfile -t $(IMAGE_REGISTRY)/ordering-service:$(IMAGE_TAG) .
	docker build -f services/ml/Dockerfile -t $(IMAGE_REGISTRY)/ml-service:$(IMAGE_TAG) services/ml

CORE_DB_URL      ?= postgres://postgres:postgres@localhost:5433/core_db?sslmode=disable
OPTIMIZATION_DB_URL ?= postgres://postgres:postgres@localhost:5434/optimization_db?sslmode=disable
ORDERING_DB_URL  ?= postgres://postgres:postgres@localhost:5435/ordering_db?sslmode=disable
DEV_STATE_DIR    ?= .dev

# ── Local infrastructure ─────────────────────────────────────────────────────

dev-infra-up:
	docker compose -f deploy/docker-compose.dev.yml up -d

dev-infra-down:
	docker compose -f deploy/docker-compose.dev.yml down

# ── Dev runners (hot-reload via air) ─────────────────────────────────────────

dev-core:
	docker compose -f deploy/docker-compose.dev.yml up -d core-db redis zookeeper kafka kafka-init minio
	@echo "Waiting for core-db..."
	@until docker compose -f deploy/docker-compose.dev.yml exec -T core-db \
	    pg_isready -U postgres -d core_db -q 2>/dev/null; do sleep 1; done
	CORE_DB_URL=$(CORE_DB_URL) $(MAKE) migrate-core
	cd services/core && swag init -g cmd/api/swagger.go -o docs/swagger --parseDependency -q
	@echo "Swagger UI: http://localhost:8081/swagger/index.html"
	cd services/core && air

dev-ordering:
	docker compose -f deploy/docker-compose.dev.yml up -d ordering-db zookeeper kafka kafka-init
	@echo "Waiting for ordering-db..."
	@until docker compose -f deploy/docker-compose.dev.yml exec -T ordering-db \
	    pg_isready -U postgres -d ordering_db -q 2>/dev/null; do sleep 1; done
	ORDERING_DB_URL=$(ORDERING_DB_URL) $(MAKE) migrate-ordering
	cd services/ordering && swag init -g cmd/api/swagger.go -o docs/swagger --parseDependency -q
	@echo "Swagger UI: http://localhost:8083/swagger/index.html"
	cd services/ordering && air

dev-all:
	docker compose -f deploy/docker-compose.dev.yml up -d core-db optimization-db ordering-db redis zookeeper kafka kafka-init minio
	@echo "Waiting for core-db..."
	@until docker compose -f deploy/docker-compose.dev.yml exec -T core-db \
	    pg_isready -U postgres -d core_db -q 2>/dev/null; do sleep 1; done
	@echo "Waiting for optimization-db..."
	@until docker compose -f deploy/docker-compose.dev.yml exec -T optimization-db \
	    pg_isready -U postgres -d optimization_db -q 2>/dev/null; do sleep 1; done
	@echo "Waiting for ordering-db..."
	@until docker compose -f deploy/docker-compose.dev.yml exec -T ordering-db \
	    pg_isready -U postgres -d ordering_db -q 2>/dev/null; do sleep 1; done
	CORE_DB_URL=$(CORE_DB_URL) $(MAKE) migrate-core
	OPTIMIZATION_DB_URL=$(OPTIMIZATION_DB_URL) $(MAKE) migrate-optimization
	ORDERING_DB_URL=$(ORDERING_DB_URL) $(MAKE) migrate-ordering
	$(MAKE) swagger
	@mkdir -p $(DEV_STATE_DIR)
	@if [ ! -d services/ml/.venv ]; then cd services/ml && $(MAKE) install; fi
	@if [ ! -f services/ml/src/proto/analogs_pb2.py ]; then cd services/ml && $(MAKE) proto; fi
	@if [ -f $(DEV_STATE_DIR)/ml.pid ] && kill -0 $$(cat $(DEV_STATE_DIR)/ml.pid) 2>/dev/null; then \
		echo "ml-service already running"; \
	else \
		(cd services/ml; nohup env CORE_GRPC_ADDR=localhost:9091 GRPC_PORT=50051 .venv/bin/python -m src.main > "$(CURDIR)/$(DEV_STATE_DIR)/ml.log" 2>&1 < /dev/null & echo $$! > "$(CURDIR)/$(DEV_STATE_DIR)/ml.pid"); \
	fi
	@if [ -f $(DEV_STATE_DIR)/core.pid ] && kill -0 $$(cat $(DEV_STATE_DIR)/core.pid) 2>/dev/null; then \
		echo "core-service already running"; \
	else \
		(cd services/core; nohup env ENV=development SERVER_PORT=8081 GRPC_PORT=9091 DB_URL="$(CORE_DB_URL)" REDIS_URL=redis://localhost:6379/0 KAFKA_BROKERS=localhost:9092 JWT_SECRET=dev-secret-change-in-prod go run ./cmd/api > "$(CURDIR)/$(DEV_STATE_DIR)/core.log" 2>&1 < /dev/null & echo $$! > "$(CURDIR)/$(DEV_STATE_DIR)/core.pid"); \
	fi
	@if [ -f $(DEV_STATE_DIR)/optimization.pid ] && kill -0 $$(cat $(DEV_STATE_DIR)/optimization.pid) 2>/dev/null; then \
		echo "optimization-service already running"; \
	else \
		(cd services/optimization; nohup env ENV=development SERVER_PORT=8082 GRPC_PORT=9094 DB_URL="$(OPTIMIZATION_DB_URL)" REDIS_URL=redis://localhost:6379/0 KAFKA_BROKERS=localhost:9092 JWT_SECRET=dev-secret-change-in-prod CORE_GRPC_ADDR=localhost:9091 ML_GRPC_ADDR=localhost:50051 go run ./cmd/api > "$(CURDIR)/$(DEV_STATE_DIR)/optimization.log" 2>&1 < /dev/null & echo $$! > "$(CURDIR)/$(DEV_STATE_DIR)/optimization.pid"); \
	fi
	@if [ -f $(DEV_STATE_DIR)/ordering.pid ] && kill -0 $$(cat $(DEV_STATE_DIR)/ordering.pid) 2>/dev/null; then \
		echo "ordering-service already running"; \
	else \
		(cd services/ordering; nohup env ENV=development SERVER_PORT=8083 GRPC_PORT=9093 DB_URL="$(ORDERING_DB_URL)" REDIS_URL=redis://localhost:6379/0 KAFKA_BROKERS=localhost:9092 JWT_SECRET=dev-secret-change-in-prod CORE_GRPC_ADDR=localhost:9091 OPTIMIZATION_GRPC_ADDR=localhost:9094 go run ./cmd/api > "$(CURDIR)/$(DEV_STATE_DIR)/ordering.log" 2>&1 < /dev/null & echo $$! > "$(CURDIR)/$(DEV_STATE_DIR)/ordering.pid"); \
	fi
	@echo "Services started in background."
	@echo "Swagger Core: http://localhost:8081/swagger/index.html"
	@echo "Swagger Optimization: http://localhost:8082/swagger/index.html"
	@echo "Swagger Ordering: http://localhost:8083/swagger/index.html"
	@echo "Logs: $(DEV_STATE_DIR)/*.log"

stop-all:
	@if [ -f $(DEV_STATE_DIR)/ordering.pid ]; then \
		kill $$(cat $(DEV_STATE_DIR)/ordering.pid) 2>/dev/null || true; \
		rm -f $(DEV_STATE_DIR)/ordering.pid; \
	fi
	@if [ -f $(DEV_STATE_DIR)/optimization.pid ]; then \
		kill $$(cat $(DEV_STATE_DIR)/optimization.pid) 2>/dev/null || true; \
		rm -f $(DEV_STATE_DIR)/optimization.pid; \
	fi
	@if [ -f $(DEV_STATE_DIR)/core.pid ]; then \
		kill $$(cat $(DEV_STATE_DIR)/core.pid) 2>/dev/null || true; \
		rm -f $(DEV_STATE_DIR)/core.pid; \
	fi
	@if [ -f $(DEV_STATE_DIR)/ml.pid ]; then \
		kill $$(cat $(DEV_STATE_DIR)/ml.pid) 2>/dev/null || true; \
		rm -f $(DEV_STATE_DIR)/ml.pid; \
	fi
	docker compose -f deploy/docker-compose.dev.yml down
	@echo "All services and infrastructure stopped."

# ── Kubernetes ───────────────────────────────────────────────────────────────

K8S_ENV ?= dev

k8s-render:
	kubectl kustomize deploy/k8s/overlays/$(K8S_ENV)

k8s-deploy:
	kubectl apply -k deploy/k8s/overlays/$(K8S_ENV)

k8s-down:
	kubectl delete -k deploy/k8s/overlays/$(K8S_ENV) --ignore-not-found

k8s-logs:
	kubectl logs -f deployment/$(SVC) -n foodsea-$(K8S_ENV)

k8s-status:
	kubectl get pods,svc,ingress -n foodsea-$(K8S_ENV)

# ── Migrations ───────────────────────────────────────────────────────────────

migrate-core:
	atlas migrate apply \
		--dir "file://services/core/migrations" \
		--url "$(CORE_DB_URL)"

migrate-optimization:
	atlas migrate apply \
		--dir "file://services/optimization/migrations" \
		--url "$(OPTIMIZATION_DB_URL)"

migrate-ordering:
	atlas migrate apply \
		--dir "file://services/ordering/migrations" \
		--url "$(ORDERING_DB_URL)"

migrate: migrate-core migrate-optimization migrate-ordering

# ── Seed ─────────────────────────────────────────────────────────────────────

seed-core:
	cd services/core && go run ./cmd/seed

# ── Atlas ────────────────────────────────────────────────────────────────────

atlas-diff-core:
	cd services/core && GOWORK=off atlas migrate diff \
		--dir "file://migrations" \
		--to "ent://ent/schema" \
		--dev-url "docker://postgres/16/dev?search_path=public"

atlas-diff-ordering:
	cd services/ordering && GOWORK=off atlas migrate diff \
		--dir "file://migrations" \
		--to "ent://ent/schema" \
		--dev-url "docker://postgres/16/dev?search_path=public"

atlas-hash:
	GOWORK=off atlas migrate hash --dir "file://services/core/migrations"

# ── Tools ────────────────────────────────────────────────────────────────────

tools:
	go install entgo.io/ent/cmd/ent@latest
	go install ariga.io/atlas/cmd/atlas@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install mvdan.cc/gofumpt@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/air-verse/air@latest

# ── All ──────────────────────────────────────────────────────────────────────

all: generate proto swagger lint test build

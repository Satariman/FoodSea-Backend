.PHONY: all build generate proto swagger swagger-optimization test test-unit test-integration test-cover test-optimization test-e2e-core test-e2e-ordering test-e2e-optimization test-go-all test-swagger-regression test-ml test-all lint \
        docker-build dev-infra-up dev-infra-down dev-core dev-ordering dev-all dev-oauth-all stop-all up down \
        k8s-render k8s-deploy k8s-down k8s-logs k8s-status \
        migrate-core migrate-optimization migrate-ordering migrate \
        visual-profiles gemini-images-test gemini-images-brand-samples gemini-images-all-brand-samples gemini-images-submit gemini-images-status gemini-images-download import-generated-images \
        rebuild-photo-search-index photo-search-rebuild photo-search-restart-ml photo-search-verify-ml photo-search-refresh \
        atlas-diff-core atlas-diff-ordering atlas-hash \
        seed-core \
        tools

ifneq (,$(wildcard .env.local))
include .env.local
endif

SWAG ?= go run github.com/swaggo/swag/cmd/swag@v1.16.3

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
	cd services/core && $(SWAG) init -g cmd/api/swagger.go -o docs/swagger --parseDependency -q
	cd services/optimization && $(SWAG) init -g cmd/api/main.go -o docs/swagger --parseDependency -q
	cd services/ordering && $(SWAG) init -g cmd/api/swagger.go -o docs/swagger --parseDependency -q

swagger-optimization:
	cd services/optimization && $(SWAG) init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal

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

# ── Product image generation ────────────────────────────────────────────────

GEMINI_IMAGE_LIMIT ?= 5
GEMINI_IMAGE_BATCH_NAME ?=
GEMINI_IMAGE_EXTRA_ARGS ?=
VISUAL_PROFILES_FORCE ?= 0
IMAGE_IMPORT_MANIFEST ?= ../../reports/generated_product_images/manifest.jsonl
IMAGE_IMPORT_STATUSES ?= likely_wrong,uncertain,no_image
IMAGE_IMPORT_EXTRA_ARGS ?=

visual-profiles:
	@if [ "$(VISUAL_PROFILES_FORCE)" = "1" ] || \
		[ ! -s reports/brand_visual_profiles.json ] || \
		[ ! -s reports/product_visual_generation_profiles.jsonl ] || \
		[ ! -s reports/product_visual_generation_profiles.csv ]; then \
		echo "Building visual profiles from DB..."; \
		python3 scripts/build_visual_generation_profiles.py; \
	else \
		echo "Using existing visual profiles (set VISUAL_PROFILES_FORCE=1 to rebuild from DB)."; \
	fi

gemini-images-test: visual-profiles
	python3 scripts/generate_product_images_gemini.py --limit $(GEMINI_IMAGE_LIMIT) $(GEMINI_IMAGE_EXTRA_ARGS)

gemini-images-brand-samples: visual-profiles
	python3 scripts/generate_product_images_gemini.py --brand-profile-sample --brand-confidence curated_known_brand,local_reference_curated $(GEMINI_IMAGE_EXTRA_ARGS)

gemini-images-all-brand-samples: visual-profiles
	python3 scripts/generate_product_images_gemini.py --brand-profile-sample $(GEMINI_IMAGE_EXTRA_ARGS)

gemini-images-submit: visual-profiles
	python3 scripts/generate_product_images_gemini.py $(GEMINI_IMAGE_EXTRA_ARGS)

gemini-images-status:
	@if [ -z "$(GEMINI_IMAGE_BATCH_NAME)" ]; then \
		echo "Set GEMINI_IMAGE_BATCH_NAME=batches/..."; \
		exit 1; \
	fi
	python3 scripts/generate_product_images_gemini.py --action status --batch-name "$(GEMINI_IMAGE_BATCH_NAME)"

gemini-images-download:
	@if [ -z "$(GEMINI_IMAGE_BATCH_NAME)" ]; then \
		echo "Set GEMINI_IMAGE_BATCH_NAME=batches/..."; \
		exit 1; \
	fi
	python3 scripts/generate_product_images_gemini.py --action download --batch-name "$(GEMINI_IMAGE_BATCH_NAME)" --wait

import-generated-images:
	cd services/core && go run ./cmd/import-generated-images --manifest "$(IMAGE_IMPORT_MANIFEST)" --statuses "$(IMAGE_IMPORT_STATUSES)" $(IMAGE_IMPORT_EXTRA_ARGS)

rebuild-photo-search-index:
	cd services/ml && $(MAKE) rebuild-photo-index

photo-search-rebuild:
	@set -e; \
	if [ ! -f "$(PHOTO_SEARCH_ENV_FILE)" ]; then \
		echo "Missing env file: $(PHOTO_SEARCH_ENV_FILE)"; \
		echo "Create it from .env.photo-search.local.example"; \
		exit 1; \
	fi; \
	set -a; . "$(PHOTO_SEARCH_ENV_FILE)"; set +a; \
	if [ -z "$${GEMINI_API_KEY}" ]; then \
		echo "GEMINI_API_KEY is required in $(PHOTO_SEARCH_ENV_FILE)"; \
		exit 1; \
	fi; \
	cd services/ml; \
	.venv/bin/python -m src.photo_search.rebuild_index

photo-search-restart-ml:
	@set -e; \
	if [ ! -f "$(PHOTO_SEARCH_ENV_FILE)" ]; then \
		echo "Missing env file: $(PHOTO_SEARCH_ENV_FILE)"; \
		echo "Create it from .env.photo-search.local.example"; \
		exit 1; \
	fi; \
	mkdir -p "$(DEV_STATE_DIR)"; \
	if [ -f "$(PHOTO_SEARCH_ML_PID_FILE)" ] && kill -0 $$(cat "$(PHOTO_SEARCH_ML_PID_FILE)") 2>/dev/null; then \
		kill $$(cat "$(PHOTO_SEARCH_ML_PID_FILE)") 2>/dev/null || true; \
		sleep 1; \
	fi; \
	: > "$(PHOTO_SEARCH_ML_LOG_FILE)"; \
	set -a; . "$(PHOTO_SEARCH_ENV_FILE)"; set +a; \
	(cd services/ml; nohup env CORE_GRPC_ADDR="$${CORE_GRPC_ADDR:-localhost:9091}" GRPC_PORT="$${GRPC_PORT:-50051}" .venv/bin/python -m src.main > "$(CURDIR)/$(PHOTO_SEARCH_ML_LOG_FILE)" 2>&1 < /dev/null & echo $$! > "$(CURDIR)/$(PHOTO_SEARCH_ML_PID_FILE)"); \
	echo "ml-service restarted, pid=$$(cat "$(PHOTO_SEARCH_ML_PID_FILE)")"

photo-search-verify-ml:
	@set -e; \
	if [ ! -f "$(PHOTO_SEARCH_ML_PID_FILE)" ]; then \
		echo "Missing pid file: $(PHOTO_SEARCH_ML_PID_FILE)"; \
		exit 1; \
	fi; \
	pid=$$(cat "$(PHOTO_SEARCH_ML_PID_FILE)"); \
	if ! kill -0 "$$pid" 2>/dev/null; then \
		echo "ml-service is not running (pid=$$pid)"; \
		echo "Last logs:"; \
		tail -n 80 "$(PHOTO_SEARCH_ML_LOG_FILE)" || true; \
		exit 1; \
	fi; \
	attempts=$(PHOTO_SEARCH_READY_TIMEOUT_SECONDS); \
	while [ "$$attempts" -gt 0 ]; do \
		if grep -q "photo search enabled with provider=" "$(PHOTO_SEARCH_ML_LOG_FILE)"; then \
			echo "OK: photo search enabled"; \
			tail -n 20 "$(PHOTO_SEARCH_ML_LOG_FILE)"; \
			exit 0; \
		fi; \
		if grep -q "photo search index not loaded" "$(PHOTO_SEARCH_ML_LOG_FILE)" || grep -q "photo search provider is not configured" "$(PHOTO_SEARCH_ML_LOG_FILE)"; then \
			echo "Photo search is not ready. Check log lines below."; \
			tail -n 80 "$(PHOTO_SEARCH_ML_LOG_FILE)"; \
			exit 1; \
		fi; \
		sleep 1; \
		attempts=$$((attempts - 1)); \
	done; \
	echo "Timeout waiting for photo search readiness ($(PHOTO_SEARCH_READY_TIMEOUT_SECONDS)s)"; \
	tail -n 80 "$(PHOTO_SEARCH_ML_LOG_FILE)"; \
	exit 1

photo-search-refresh: photo-search-rebuild photo-search-restart-ml photo-search-verify-ml
	@echo "Photo-search index rebuilt, ml-service restarted, readiness verified."

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
PHOTO_SEARCH_ENV_FILE ?= .env.photo-search.local
PHOTO_SEARCH_ML_LOG_FILE ?= $(DEV_STATE_DIR)/ml.log
PHOTO_SEARCH_ML_PID_FILE ?= $(DEV_STATE_DIR)/ml.pid
PHOTO_SEARCH_READY_TIMEOUT_SECONDS ?= 45
OAUTH_STATE_TTL  ?= 10m
OAUTH_ALLOWED_REDIRECT_URIS ?= http://localhost:3000/oauth/callback
OAUTH_NATIVE_ALLOWED_REDIRECT_URIS ?= app://foodsea/oauth/callback,http://localhost:3000/oauth/callback
OAUTH_LEGACY_ENABLED ?= true
OAUTH_NATIVE_ENABLED ?= true

OAUTH_GOOGLE_ENABLED ?= false
OAUTH_GOOGLE_CLIENT_ID ?=
OAUTH_GOOGLE_CLIENT_SECRET ?=
OAUTH_GOOGLE_AUTH_URL ?= https://accounts.google.com/o/oauth2/v2/auth
OAUTH_GOOGLE_TOKEN_URL ?= https://oauth2.googleapis.com/token
OAUTH_GOOGLE_SCOPES ?= openid,email,profile
OAUTH_GOOGLE_NATIVE_CLIENT_ID ?=
OAUTH_GOOGLE_NATIVE_CLIENT_SECRET ?=
OAUTH_GOOGLE_NATIVE_AUTH_URL ?= https://accounts.google.com/o/oauth2/v2/auth
OAUTH_GOOGLE_NATIVE_TOKEN_URL ?= https://oauth2.googleapis.com/token
OAUTH_GOOGLE_NATIVE_SCOPES ?= openid,email,profile

OAUTH_YANDEX_ENABLED ?= false
OAUTH_YANDEX_CLIENT_ID ?=
OAUTH_YANDEX_CLIENT_SECRET ?=
OAUTH_YANDEX_AUTH_URL ?= https://oauth.yandex.ru/authorize
OAUTH_YANDEX_TOKEN_URL ?= https://oauth.yandex.ru/token
OAUTH_YANDEX_USERINFO_URL ?= https://login.yandex.ru/info
OAUTH_YANDEX_SCOPES ?= login:email,login:avatar
OAUTH_YANDEX_NATIVE_SDK_ENABLED ?= true

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
	cd services/core && $(SWAG) init -g cmd/api/swagger.go -o docs/swagger --parseDependency -q
	@echo "Swagger UI: http://localhost:8081/swagger/index.html"
	cd services/core && air

dev-ordering:
	docker compose -f deploy/docker-compose.dev.yml up -d ordering-db zookeeper kafka kafka-init
	@echo "Waiting for ordering-db..."
	@until docker compose -f deploy/docker-compose.dev.yml exec -T ordering-db \
	    pg_isready -U postgres -d ordering_db -q 2>/dev/null; do sleep 1; done
	ORDERING_DB_URL=$(ORDERING_DB_URL) $(MAKE) migrate-ordering
	cd services/ordering && $(SWAG) init -g cmd/api/swagger.go -o docs/swagger --parseDependency -q
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
		(cd services/core; nohup env ENV=development SERVER_PORT=8081 GRPC_PORT=9091 DB_URL="$(CORE_DB_URL)" REDIS_URL=redis://localhost:6379/0 KAFKA_BROKERS=localhost:9092 JWT_SECRET=dev-secret-change-in-prod OAUTH_STATE_TTL="$(OAUTH_STATE_TTL)" OAUTH_ALLOWED_REDIRECT_URIS="$(OAUTH_ALLOWED_REDIRECT_URIS)" OAUTH_NATIVE_ALLOWED_REDIRECT_URIS="$(OAUTH_NATIVE_ALLOWED_REDIRECT_URIS)" OAUTH_LEGACY_ENABLED="$(OAUTH_LEGACY_ENABLED)" OAUTH_NATIVE_ENABLED="$(OAUTH_NATIVE_ENABLED)" OAUTH_GOOGLE_ENABLED="$(OAUTH_GOOGLE_ENABLED)" OAUTH_GOOGLE_CLIENT_ID="$(OAUTH_GOOGLE_CLIENT_ID)" OAUTH_GOOGLE_CLIENT_SECRET="$(OAUTH_GOOGLE_CLIENT_SECRET)" OAUTH_GOOGLE_AUTH_URL="$(OAUTH_GOOGLE_AUTH_URL)" OAUTH_GOOGLE_TOKEN_URL="$(OAUTH_GOOGLE_TOKEN_URL)" OAUTH_GOOGLE_SCOPES="$(OAUTH_GOOGLE_SCOPES)" OAUTH_GOOGLE_NATIVE_CLIENT_ID="$(OAUTH_GOOGLE_NATIVE_CLIENT_ID)" OAUTH_GOOGLE_NATIVE_CLIENT_SECRET="$(OAUTH_GOOGLE_NATIVE_CLIENT_SECRET)" OAUTH_GOOGLE_NATIVE_AUTH_URL="$(OAUTH_GOOGLE_NATIVE_AUTH_URL)" OAUTH_GOOGLE_NATIVE_TOKEN_URL="$(OAUTH_GOOGLE_NATIVE_TOKEN_URL)" OAUTH_GOOGLE_NATIVE_SCOPES="$(OAUTH_GOOGLE_NATIVE_SCOPES)" OAUTH_YANDEX_ENABLED="$(OAUTH_YANDEX_ENABLED)" OAUTH_YANDEX_CLIENT_ID="$(OAUTH_YANDEX_CLIENT_ID)" OAUTH_YANDEX_CLIENT_SECRET="$(OAUTH_YANDEX_CLIENT_SECRET)" OAUTH_YANDEX_AUTH_URL="$(OAUTH_YANDEX_AUTH_URL)" OAUTH_YANDEX_TOKEN_URL="$(OAUTH_YANDEX_TOKEN_URL)" OAUTH_YANDEX_USERINFO_URL="$(OAUTH_YANDEX_USERINFO_URL)" OAUTH_YANDEX_SCOPES="$(OAUTH_YANDEX_SCOPES)" OAUTH_YANDEX_NATIVE_SDK_ENABLED="$(OAUTH_YANDEX_NATIVE_SDK_ENABLED)" go run ./cmd/api > "$(CURDIR)/$(DEV_STATE_DIR)/core.log" 2>&1 < /dev/null & echo $$! > "$(CURDIR)/$(DEV_STATE_DIR)/core.pid"); \
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

dev-oauth-all:
	@if [ -z "$(OAUTH_GOOGLE_CLIENT_ID)" ]; then echo "OAUTH_GOOGLE_CLIENT_ID is required for dev-oauth-all"; exit 1; fi
	@if [ -z "$(OAUTH_GOOGLE_CLIENT_SECRET)" ]; then echo "OAUTH_GOOGLE_CLIENT_SECRET is required for dev-oauth-all"; exit 1; fi
	@if [ -z "$(OAUTH_GOOGLE_NATIVE_CLIENT_ID)" ]; then echo "OAUTH_GOOGLE_NATIVE_CLIENT_ID is required for dev-oauth-all"; exit 1; fi
	@if [ -z "$(OAUTH_YANDEX_CLIENT_ID)" ]; then echo "OAUTH_YANDEX_CLIENT_ID is required for dev-oauth-all"; exit 1; fi
	@if [ -z "$(OAUTH_YANDEX_CLIENT_SECRET)" ]; then echo "OAUTH_YANDEX_CLIENT_SECRET is required for dev-oauth-all"; exit 1; fi
	OAUTH_GOOGLE_ENABLED=true \
	OAUTH_NATIVE_ENABLED=true \
	OAUTH_LEGACY_ENABLED=true \
	OAUTH_YANDEX_ENABLED=true \
	$(MAKE) dev-all

up: dev-all

down: stop-all

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

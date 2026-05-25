## pulse-platform — top-level Makefile
## Run 'make help' to list all available targets.

SHELL := /usr/bin/env bash
.ONESHELL:

PROTO_DIR    := proto
GO_SERVICES  := user-service order-service inventory-service payment-service notification-service
ALL_SERVICES := $(GO_SERVICES) api-gateway

.PHONY: help proto build test lint docker-build dev-up dev-down dev-bootstrap run-all $(GO_SERVICES)

# ─── Help ────────────────────────────────────────────────────────────────────
## help: Print this help message
help:
	@grep -E '^## [a-zA-Z_-]+:' $(MAKEFILE_LIST) | \
	  sed 's/^## //' | \
	  awk -F: '{ printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }'

# ─── Proto codegen ───────────────────────────────────────────────────────────
## proto: Regenerate gRPC stubs for all Go services from proto/ definitions
proto:
	@which protoc          >/dev/null || { echo "ERROR: protoc not found"; exit 1; }
	@which protoc-gen-go   >/dev/null || { echo "ERROR: protoc-gen-go not found — run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1; }
	@which protoc-gen-go-grpc >/dev/null || { echo "ERROR: protoc-gen-go-grpc not found — run: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; exit 1; }
	@for svc in user order inventory payment; do \
	  OUT_DIR="services/$${svc}-service/gen/pb"; \
	  mkdir -p "$${OUT_DIR}"; \
	  echo "  Generating stubs for $${svc}.proto → $${OUT_DIR}"; \
	  protoc \
	    --proto_path=$(PROTO_DIR) \
	    --go_out="$${OUT_DIR}" \
	    --go_opt=paths=source_relative \
	    --go-grpc_out="$${OUT_DIR}" \
	    --go-grpc_opt=paths=source_relative \
	    $(PROTO_DIR)/$${svc}.proto; \
	done
	@echo "proto codegen complete."

# ─── Build ───────────────────────────────────────────────────────────────────
## build: Compile all Go services and transpile api-gateway TypeScript
build:
	@echo "==> Building Go services..."
	@for svc in $(GO_SERVICES); do \
	  echo "  go build services/$$svc"; \
	  (cd services/$$svc && go build ./...); \
	done
	@echo "==> Building api-gateway (tsc)..."
	(cd services/api-gateway && npm run build)
	@echo "Build complete."

# ─── Test ────────────────────────────────────────────────────────────────────
## test: Run unit tests for all services (Go race detector + api-gateway jest)
test:
	@echo "==> Testing Go services..."
	@for svc in $(GO_SERVICES); do \
	  echo "  go test services/$$svc"; \
	  (cd services/$$svc && go test ./... -race -coverprofile=coverage.out -covermode=atomic); \
	done
	@echo "==> Testing api-gateway..."
	(cd services/api-gateway && npm test)
	@echo "All tests passed."

# ─── Lint ────────────────────────────────────────────────────────────────────
## lint: Run golangci-lint on Go services and ESLint on api-gateway
lint:
	@echo "==> Linting Go services..."
	@for svc in $(GO_SERVICES); do \
	  echo "  golangci-lint services/$$svc"; \
	  (cd services/$$svc && golangci-lint run ./...); \
	done
	@echo "==> Linting api-gateway..."
	(cd services/api-gateway && npm run lint)
	@echo "Lint complete."

# ─── Docker ──────────────────────────────────────────────────────────────────
## docker-build: Build Docker images for all services (tagged :local)
docker-build:
	@echo "==> Building Docker images..."
	@for svc in $(ALL_SERVICES); do \
	  echo "  docker build services/$$svc → pulse-platform/$$svc:local"; \
	  docker build -t pulse-platform/$$svc:local services/$$svc; \
	done
	@echo "Docker images built."

# ─── Dev environment ─────────────────────────────────────────────────────────
## dev-up: Start local infrastructure (Kafka, Postgres, Redis, Elasticsearch)
dev-up:
	docker compose -f docker-compose.dev.yml up -d

## dev-down: Stop and remove local infrastructure containers
dev-down:
	docker compose -f docker-compose.dev.yml down

## dev-bootstrap: Start infrastructure, wait for Kafka, create topics
dev-bootstrap: dev-up
	bash scripts/bootstrap.sh

# ─── Run all services locally ────────────────────────────────────────────────
## run-all: Start all services in the background (requires local infra running)
run-all:
	@echo "==> Starting all services in background..."
	@for svc in $(GO_SERVICES); do \
	  echo "  Starting $$svc..."; \
	  (cd services/$$svc && go run ./... &); \
	done
	@echo "  Starting api-gateway..."
	(cd services/api-gateway && npm run dev &)
	@echo ""
	@echo "All services started. API Gateway: http://localhost:3000"
	@echo "To stop: kill %% or pkill -f 'go run'"

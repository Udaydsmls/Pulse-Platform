## Pulse Platform — run 'make help' to list targets.

GO_SERVICES  := user-service order-service inventory-service payment-service notification-service
ALL_SERVICES := $(GO_SERVICES) api-gateway

# Services with a .proto contract. payment and notification are pure Kafka
# consumers with no gRPC API, so they have no proto to generate.
PROTO_SERVICES := user order inventory

.PHONY: help setup proto deps build test lint docker-build dev-up dev-down dev-bootstrap seed run-all

## help: List available targets
help:
	@grep -E '^## [a-zA-Z_-]+:' $(MAKEFILE_LIST) | sed 's/^## //' | \
	  awk -F': ' '{ printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2 }'

# ─── One-time setup ──────────────────────────────────────────────────────────
## setup: Generate protobuf code and resolve Go dependencies (run this first)
setup: proto deps

## proto: Generate Go gRPC code from proto/ into each service's gen/pb/
proto:
	@command -v protoc >/dev/null || { echo "ERROR: protoc not installed"; exit 1; }
	@command -v protoc-gen-go >/dev/null || { echo "ERROR: run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1; }
	@command -v protoc-gen-go-grpc >/dev/null || { echo "ERROR: run: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; exit 1; }
	@for svc in $(PROTO_SERVICES); do \
	  out="services/$$svc-service/gen/pb"; \
	  mkdir -p "$$out"; \
	  echo "  proto/$$svc.proto -> $$out"; \
	  protoc --proto_path=proto \
	    --go_out="$$out" --go_opt=paths=source_relative \
	    --go-grpc_out="$$out" --go-grpc_opt=paths=source_relative \
	    "proto/$$svc.proto" || exit 1; \
	done
	@echo "proto generation complete."

## deps: Resolve Go modules and write go.sum for each service
deps:
	@for svc in $(GO_SERVICES); do \
	  echo "  go mod tidy services/$$svc"; \
	  (cd services/$$svc && go mod tidy) || exit 1; \
	done
	@echo "  npm install services/api-gateway"
	@cd services/api-gateway && npm install

# ─── Build, test, lint ───────────────────────────────────────────────────────
## build: Compile every service
build:
	@for svc in $(GO_SERVICES); do \
	  echo "  go build services/$$svc"; \
	  (cd services/$$svc && go build ./...) || exit 1; \
	done
	@echo "  tsc services/api-gateway"
	@cd services/api-gateway && npm run build

## test: Run unit tests for every service
test:
	@for svc in $(GO_SERVICES); do \
	  echo "  go test services/$$svc"; \
	  (cd services/$$svc && go test ./... -race) || exit 1; \
	done
	@echo "  jest services/api-gateway"
	@cd services/api-gateway && npm test

## lint: Run golangci-lint and eslint
lint:
	@for svc in $(GO_SERVICES); do \
	  echo "  golangci-lint services/$$svc"; \
	  (cd services/$$svc && golangci-lint run ./...) || exit 1; \
	done
	@cd services/api-gateway && npm run lint

## docker-build: Build all Docker images tagged :local
docker-build:
	@for svc in $(GO_SERVICES); do \
	  echo "  building pulse-platform/$$svc:local"; \
	  docker build -t pulse-platform/$$svc:local services/$$svc || exit 1; \
	done
	@# The gateway builds from the repo root so it can copy in proto/.
	docker build -f services/api-gateway/Dockerfile -t pulse-platform/api-gateway:local .

# ─── Local development ───────────────────────────────────────────────────────
## dev-up: Start Kafka, Postgres and Redis
dev-up:
	docker compose -f docker-compose.dev.yml up -d

## dev-down: Stop local infrastructure
dev-down:
	docker compose -f docker-compose.dev.yml down

## dev-bootstrap: Start infrastructure and create Kafka topics
dev-bootstrap:
	bash scripts/bootstrap.sh

## seed: Seed stock and walk an order through both saga paths
seed:
	bash scripts/seed_data.sh

## run-all: Run every service locally in the background
run-all:
	@echo "Starting services. Stop them with: pkill -f 'go run'"
	@for svc in $(GO_SERVICES); do \
	  echo "  $$svc"; \
	  (cd services/$$svc && go run . &); \
	done
	@cd services/api-gateway && npm run dev

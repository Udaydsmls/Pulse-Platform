# Pulse Platform

A production-grade e-commerce microservices platform demonstrating gRPC inter-service communication, event-driven sagas via Apache Kafka, Kubernetes deployment with Istio service mesh, and full observability.

## Architecture

Six microservices communicate via gRPC (synchronous) and Kafka (asynchronous). An API gateway is the single internet-facing entry point. Istio handles mTLS, traffic policy, and circuit breaking across the mesh.

```
                         ┌─────────────────────────────────────┐
Internet ──► Nginx ──► api-gateway (Node.js/TS)               │
                         │  REST + GraphQL + WebSocket          │
                         │  Rate limiting (Redis)               │
                         └──┬─────────────┬───────────┬────────┘
                            │ gRPC        │ gRPC      │ gRPC
                    ┌───────▼──┐  ┌───────▼──┐  ┌────▼──────────┐
                    │user-svc  │  │order-svc │  │inventory-svc  │
                    │(Go/PG)   │  │(Go/PG)   │  │(Go/PG)        │
                    └──────────┘  └──────────┘  └───────────────┘
                                                        │
                    Kafka topics: user.events, order.events,
                    inventory.events, payment.events, notification.events
                                                        │
                    ┌───────────────┐  ┌────────────────┴──┐
                    │payment-svc    │  │notification-svc   │
                    │(Go/PG/Stripe) │  │(Go/DynamoDB)      │
                    └───────────────┘  └───────────────────┘
```

### Saga Pattern (Order Flow)

```
CreateOrder ──► order.created ──► inventory-service
                                       │
                            ┌──────────┴──────────┐
                    inventory.reserved      inventory.failed
                            │                      │
                    payment-service          order.cancelled
                            │
                 ┌──────────┴──────────┐
          payment.confirmed      payment.failed
                 │                      │
          order.confirmed         order.cancelled
                                  + inventory release
```

## Repository Structure

```
pulse-platform/
├── services/
│   ├── user-service/           # Go — auth, JWT, OAuth2 (Google/GitHub)
│   ├── order-service/          # Go — order lifecycle + saga orchestration
│   ├── inventory-service/      # Go — stock management, reservations
│   ├── payment-service/        # Go — Stripe mock, payment saga step
│   ├── notification-service/   # Go — event fan-out, email/SMS, DynamoDB log
│   └── api-gateway/            # TypeScript — REST, GraphQL, WebSocket
├── proto/                      # Shared .proto contract files
├── infra/terraform/            # AWS VPC, EKS, RDS, MSK, ElastiCache, ECR
├── infra/cdk/                  # EKS node groups, namespaces, External Secrets
├── helm/                       # One Helm chart per service
├── k8s/                        # Istio VirtualService, DestinationRule, PeerAuthentication
├── observability/              # Prometheus, Grafana dashboards, OTel Collector
├── scripts/                    # Local bootstrap, Kafka topic creation, seed data
├── load_tests/                 # k6 load tests (500 VUs, p95 < 200ms threshold)
├── docker-compose.dev.yml      # Local: Kafka, Zookeeper, PostgreSQL, Redis, Elasticsearch
└── .github/workflows/          # CI per service: test, lint, build, Docker push, Snyk
```

## Services

| Service | Language | Port | Database | Publishes | Consumes |
|---|---|---|---|---|---|
| user-service | Go | 50051 | PostgreSQL | user.events | — |
| order-service | Go | 50052 | PostgreSQL | order.events | inventory.events, payment.events |
| inventory-service | Go | 50052 | PostgreSQL | inventory.events | order.events |
| payment-service | Go | 50053 | PostgreSQL | payment.events | inventory.events |
| notification-service | Go | — | DynamoDB | — | all topics |
| api-gateway | TypeScript | 3000 | Redis (cache) | — | notification.events (WS relay) |

## Technology Stack

| Concern | Technology |
|---|---|
| Service communication | gRPC (protobuf) |
| Async messaging | Apache Kafka (MSK in prod) |
| Service mesh | Istio (mTLS STRICT, circuit breaking) |
| API gateway | Express + Apollo Server 4 + WebSocket |
| Auth | JWT + OAuth2 (Google, GitHub) |
| Rate limiting | Redis sliding window |
| Search | Elasticsearch (Kafka → ES pipeline) |
| Container orchestration | Kubernetes (EKS) |
| Package management (K8s) | Helm |
| Ingress | Nginx Ingress Controller |
| Infrastructure | Terraform (AWS) + AWS CDK |
| Secrets | AWS Secrets Manager + External Secrets Operator + HashiCorp Vault |
| Observability | Prometheus + Grafana + OpenTelemetry + Jaeger + Datadog |
| CI/CD | GitHub Actions |
| Security scanning | Snyk + OWASP Dependency-Check |
| Load testing | k6 |
| Chaos engineering | chaoskube |

## Prerequisites

- Go 1.22+
- Node.js 20+
- Docker + Docker Compose
- `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` (for proto generation)
- `kubectl`, `helm`, `istioctl` (for Kubernetes deployment)
- `terraform` >= 1.8, AWS CDK v2 (for infrastructure)

## Local Development

### 1. Generate protobuf code

This must be done before building any Go service:

```bash
make proto
```

This runs `protoc` for each service and outputs generated Go types to `services/<name>/gen/pb/`.

### 2. Start local infrastructure

```bash
make dev-bootstrap
# Equivalent to:
docker compose -f docker-compose.dev.yml up -d
./scripts/create_topics.sh
```

This starts: Kafka + Zookeeper, PostgreSQL, Redis, Elasticsearch.

### 3. Run services

```bash
make run-all
# Or individually:
cd services/user-service && go run cmd/server/main.go
cd services/order-service && go run cmd/server/main.go
cd services/inventory-service && go run cmd/server/main.go
cd services/payment-service && go run cmd/server/main.go
cd services/notification-service && go run cmd/server/main.go
cd services/api-gateway && npm install && npm run dev
```

### 4. Seed data

```bash
./scripts/seed_data.sh
```

### Endpoints

- REST API: `http://localhost:3000`
- GraphQL Playground: `http://localhost:3000/graphql`
- WebSocket (order updates): `ws://localhost:3000/ws/orders?token=<jwt>`

### Environment variables

Each service reads configuration from environment variables. Key variables:

| Variable | Services | Description |
|---|---|---|
| `DATABASE_URL` | user, order, inventory, payment | PostgreSQL connection string |
| `KAFKA_BROKERS` | all Go services | Comma-separated broker list |
| `GRPC_PORT` | Go services | gRPC listen port |
| `JWT_SECRET` | user-service, api-gateway | JWT signing key |
| `REDIS_URL` | api-gateway | Redis connection string |
| `ELASTICSEARCH_URL` | api-gateway | Elasticsearch endpoint |
| `OTEL_ENDPOINT` | all services | OpenTelemetry Collector gRPC endpoint |
| `DD_API_KEY` | otel-collector | Datadog API key |

See each service's `internal/config/config.go` for the full list.

## Production Deployment

### 1. Provision infrastructure

```bash
cd infra/terraform
terraform init
terraform apply
```

### 2. Deploy application layer

```bash
cd infra/cdk
npm install
cdk deploy
```

### 3. Install Istio

```bash
istioctl install --set profile=default
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/istio/
```

### 4. Install observability stack

```bash
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace
kubectl apply -f observability/otel/otel-collector.yaml
kubectl apply -f observability/grafana/
```

### 5. Deploy services

For each service:

```bash
# Build and push
docker build -t <ECR_URL>/pulse-platform/<service>:$TAG services/<service>/
docker push <ECR_URL>/pulse-platform/<service>:$TAG

# Deploy
helm upgrade --install <service> helm/<service>/ \
  -f helm/<service>/values.prod.yaml \
  --set image.tag=$TAG \
  --namespace pulse-platform
```

### 6. Apply ingress

```bash
kubectl apply -f k8s/ingress.yaml
```

## CI/CD

Each service has a dedicated GitHub Actions workflow (`.github/workflows/<service>.yml`) that runs on push/PR to the service's path:

1. **test** — unit tests with race detector
2. **lint** — golangci-lint (Go) / eslint (TypeScript)
3. **build** — compile + Docker build; push to ECR on `main`
4. **snyk** — dependency vulnerability scan

A weekly **security-scan** workflow runs OWASP Dependency-Check across all services and uploads SARIF reports to GitHub Code Scanning.

## Load Testing

```bash
k6 run load_tests/api_load_test.js
# Override base URL:
BASE_URL=https://api.pulse-platform.example.com k6 run load_tests/api_load_test.js
```

Ramps to 500 virtual users. Thresholds: p95 latency < 200ms, error rate < 1%.

## Observability

Three Grafana dashboards are pre-built in `observability/grafana/`:

| Dashboard | Key panels |
|---|---|
| Service Overview | RPS, p95 latency, error rate per service |
| Kafka Consumer Lag | Lag per topic/consumer group |
| Kubernetes Cluster Health | CPU, memory, pod restarts by node/namespace |

Traces flow: Go service → OTel SDK → OTel Collector → Jaeger + Datadog.
Metrics flow: Go service `/metrics` → Prometheus → Grafana.

## WebSocket — Real-time Order Updates

The full event pipeline for real-time updates:

```
Order event (Kafka) ──► api-gateway consumer
                              │
                        Redis pub/sub
                        (order_updates:{userId})
                              │
                        WebSocket server ──► browser client
```

Connect at `ws://localhost:3000/ws/orders?token=<jwt>`. The server pushes JSON order status updates as they occur.

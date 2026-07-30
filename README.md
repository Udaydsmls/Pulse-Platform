# Pulse Platform

An e-commerce microservices platform: five Go services and a TypeScript API
gateway, coordinating orders with the saga pattern over Kafka, deployed to AWS
EKS with Helm and Istio.

## Architecture

```
                 Internet
                     │
              Nginx Ingress
                     │
        ┌────────────▼─────────────┐
        │       api-gateway        │   TypeScript
        │  REST · GraphQL · WS     │   port 3000
        └──┬──────┬──────────┬─────┘
           │ gRPC │ gRPC     │ gRPC
    ┌──────▼──┐ ┌─▼───────┐ ┌▼──────────────┐
    │  user   │ │  order  │ │   inventory   │   Go + Postgres
    │  50051  │ │  50052  │ │     50053     │
    └─────────┘ └─────────┘ └───────────────┘
                     │
        Kafka: user.events, order.events,
        inventory.events, payment.events,
        notification.events
                     │
        ┌────────────┴──────┐
    ┌───▼──────┐  ┌─────────▼────────┐
    │ payment  │  │  notification    │   Go, Kafka only
    │          │  │                  │   Postgres / DynamoDB
    └──────────┘  └──────────────────┘
```

gRPC handles synchronous request/response from the gateway. Kafka carries
everything between the services, which is what lets the order flow be a saga
rather than a chain of blocking calls. payment-service and notification-service
have no gRPC API at all — they react to events and announce results, so Kafka is
their only interface.

Istio provides mTLS and circuit breaking inside the mesh.

### The order saga

Each service does one step and announces the result. `order-service` listens for
those results and moves the order along:

```
CreateOrder ──► order.created
                     │
                     ▼
            inventory-service reserves stock
                     │
          ┌──────────┴───────────┐
   inventory.reserved      inventory.failed
          │                      │
          ▼                      │
  payment-service charges        │
          │                      │
   ┌──────┴───────┐              │
payment.confirmed  payment.failed│
      │                 │        │
      ▼                 ▼        ▼
order.confirmed      order.cancelled
                            │
                            ▼
              inventory-service releases stock
```

There is no distributed transaction to roll back, so **every failure step
publishes `order.cancelled` as its compensating event**, and `inventory-service`
releases whatever it had reserved. A customer-initiated cancel takes the same
path.

Releases are keyed by order ID and only touch reservations still marked
`active`, so a duplicate `order.cancelled` is a no-op rather than a double
release. That is what makes the saga safe under Kafka's at-least-once delivery
and under `chaoskube` killing pods mid-flow.

### Real-time order updates

```
notification.events (Kafka)
        │
        ▼
api-gateway consumer
        │
        ▼
Redis pub/sub  (channel: order_updates:{userId})
        │
        ▼
WebSocket ──► browser
```

Redis sits in the middle so it doesn't matter which gateway replica holds the
customer's socket: whichever replica consumed the Kafka message publishes to the
channel, and whichever one holds the socket is subscribed to it.

```
ws://localhost:3000/ws/orders?token=<jwt>
```

## Services

| Service | gRPC port | Storage | Publishes | Consumes |
|---|---|---|---|---|
| user-service | 50051 | Postgres | `user.events` | — |
| order-service | 50052 | Postgres | `order.events` | `inventory.events`, `payment.events` |
| inventory-service | 50053 | Postgres | `inventory.events` | `order.events` |
| payment-service | — | Postgres | `payment.events` | `inventory.events` |
| notification-service | — | DynamoDB | `notification.events` | all four above |
| api-gateway | 3000 (HTTP) | Redis | — | `notification.events` |

Every Go service also serves Prometheus metrics on `:9090/metrics`.

### The event contract

All services put the same flat `Event` struct on the wire — one shape with
optional fields, rather than a payload type per event:

```json
{
  "event_type": "order.created",
  "order_id": "...",
  "user_id": "...",
  "email": "...",
  "items": [{ "product_id": "p1", "quantity": 2, "unit_price": 29.99 }],
  "total": 59.98,
  "timestamp": "..."
}
```

It's deliberately one struct duplicated across the services rather than a shared
module: each service is its own Go module, and a single shape means a producer
and consumer can't quietly disagree about field names. A service omits the
fields it never reads — unknown JSON fields are dropped on decode.

## Layout

```
services/            One flat Go package per service (no internal/ nesting)
proto/               gRPC contracts for the three services that have one
infra/terraform/     VPC, EKS, RDS, MSK, ElastiCache, ECR
infra/cdk/           EKS node groups, namespaces, External Secrets
helm/                One chart per service
k8s/                 Istio mTLS + circuit breaking, ingress, chaoskube
observability/       Prometheus, Grafana dashboards, OTel Collector
load_tests/          k6 (500 VUs, p95 < 200 ms)
scripts/             Local bootstrap, topic creation, seed + saga demo
```

## Running it locally

### 1. Generate code and resolve dependencies

```bash
make setup
```

This runs `protoc` into each service's `gen/pb/` and then `go mod tidy` /
`npm install`. **It has to run first** — the generated protobuf code is not
committed, so nothing compiles until it exists.

Needs `protoc` plus the two Go plugins:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### 2. Start infrastructure

```bash
make dev-bootstrap
```

Starts Kafka, Postgres and Redis, waits for the broker, and creates the topics.
Auto-creation is off, so this step matters.

### 3. Run the services

```bash
make run-all
```

Each service creates its own tables on first start. Required environment
variables are validated at startup — a missing `JWT_SECRET` is a hard failure,
not a default, since an empty signing key produces forgeable tokens.

```bash
export DATABASE_URL="postgresql://pulse:pulse@localhost:5432/pulsedb"
export JWT_SECRET="dev-secret-change-me"
export REDIS_URL="redis://localhost:6379"
```

### 4. Watch the saga run

```bash
make seed
```

Seeds stock, then places two orders: one that succeeds, and one over the mock
payment gateway's decline limit. The second prints the stock level before and
after so you can see the compensating event put it back.

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | — | Postgres, required by user/order/inventory/payment |
| `JWT_SECRET` | — | Required by user-service and api-gateway |
| `REDIS_URL` | — | Required by api-gateway |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated broker list |
| `GRPC_PORT` | per service | See the table above |
| `METRICS_PORT` | `9090` | Prometheus endpoint |
| `OTEL_ENDPOINT` | unset | OTel Collector; tracing is off when unset |
| `PAYMENT_DECLINE_OVER` | `5000` | Mock gateway declines above this |
| `DYNAMODB_TABLE` | `pulse-notifications` | notification-service log table |

### Endpoints

| | |
|---|---|
| REST | `http://localhost:3000` |
| GraphQL | `http://localhost:3000/graphql` |
| WebSocket | `ws://localhost:3000/ws/orders?token=<jwt>` |
| Health | `http://localhost:3000/health` |

`POST /auth/register`, `POST /auth/login`, `POST /auth/logout`,
`GET|POST /orders`, `DELETE /orders/:id`, `GET /products`,
`GET /products/:id/stock`.

## Testing

```bash
make test    # go test -race per service, jest for the gateway
make lint    # golangci-lint + eslint
```

## Deployment

```bash
# 1. Infrastructure
cd infra/terraform && terraform init && terraform apply
cd ../cdk && npm install && cdk deploy

# 2. Istio
istioctl install --set profile=default
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/istio/          # mTLS STRICT + circuit breaking

# 3. Observability
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace
kubectl apply -f observability/otel/otel-collector.yaml
kubectl apply -f observability/grafana/

# 4. Services
helm upgrade --install <service> helm/<service>/ \
  -f helm/<service>/values.prod.yaml \
  --set image.tag=$TAG --namespace pulse-platform

# 5. Ingress and chaos testing
kubectl apply -f k8s/ingress.yaml
kubectl apply -f k8s/chaoskube.yaml
```

Secrets come from AWS Secrets Manager via External Secrets, which syncs them
into a `<service>-secrets` Kubernetes secret that each chart mounts with
`envFrom`. The secret keys are the environment variable names the services
actually read, so `$(DB_PASSWORD)` in a chart resolves.

Readiness and liveness probes are TCP checks rather than gRPC ones: these
services don't register `grpc.health.v1.Health`, and a `grpc` probe against a
server that lacks it never passes.

## Observability

Traces go from each service's OTel SDK to the OTel Collector, which fans out to
Jaeger and Datadog. Metrics are scraped by Prometheus from `:9090/metrics` —
pods are discovered by the `prometheus.io/scrape` annotation the charts set.

Three Grafana dashboards are in `observability/grafana/`:

| Dashboard | Panels |
|---|---|
| Service Overview | RPS, p95 latency and error rate per service |
| Kafka Consumer Lag | Lag per topic and consumer group |
| Kubernetes Cluster Health | CPU, memory and pod restarts |

The service overview reads `grpc_server_handled_total` and
`grpc_server_handling_seconds_bucket`, which come from the gRPC Prometheus
interceptor the three gRPC services install.

## Load testing

```bash
k6 run load_tests/api_load_test.js
```

Ramps to 500 VUs with thresholds of p95 < 200 ms and an error rate under 1%. Run
`make seed` first so the products have stock.

## Chaos testing

`k8s/chaoskube.yaml` kills a random pod in the namespace every 10 minutes during
weekday business hours, excluding itself and the gateway. The saga is designed to
survive it: Kafka offsets mean a service that dies mid-flow resumes where it left
off, and the compensating events are idempotent.

Start with `--dry-run=true` and read the logs before letting it delete anything.

## Technology

| Concern | Choice |
|---|---|
| Sync communication | gRPC (protobuf) |
| Async messaging | Kafka (MSK in production) |
| Service mesh | Istio — mTLS STRICT, circuit breaking |
| Gateway | Express + Apollo Server 4 + ws |
| Auth | JWT (HS256), bcrypt password hashing |
| Databases | Postgres (RDS), DynamoDB |
| Cache and pub/sub | Redis (ElastiCache) |
| Orchestration | Kubernetes (EKS) + Helm |
| Infrastructure | Terraform + AWS CDK |
| Secrets | AWS Secrets Manager + External Secrets |
| Observability | OpenTelemetry → Prometheus / Grafana / Jaeger / Datadog |
| CI/CD | GitHub Actions → ECR |
| Security scanning | Snyk + OWASP Dependency-Check |
| Load testing | k6 |
| Chaos engineering | chaoskube |

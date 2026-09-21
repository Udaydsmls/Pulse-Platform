# Pulse

A small e-commerce backend built as microservices: five Go services and a
TypeScript API gateway. Orders are processed asynchronously over Kafka using
the saga pattern, so no service has to wait on another.

## Architecture

```
                 client
                    │
                    ▼
            ┌───────────────┐
            │  api-gateway  │  TypeScript, port 3000
            └───┬───┬───┬───┘
       HTTP     │   │   │
    ┌───────────┘   │   └───────────┐
    ▼               ▼               ▼
┌────────┐   ┌───────────┐   ┌─────────────┐
│  user  │   │   order   │   │  inventory  │   Go + Postgres
│  8081  │   │   8082    │   │    8083     │
└────────┘   └───────────┘   └─────────────┘
                    │
     Kafka topics: user.events, order.events,
          inventory.events, payment.events
                    │
        ┌───────────┴────────────┐
        ▼                        ▼
┌───────────────┐      ┌──────────────────┐
│    payment    │      │   notification   │   Go, Kafka only
└───────────────┘      └──────────────────┘
```

The gateway calls the three services that have an HTTP API. Everything between
the services goes over Kafka. payment-service and notification-service have no
API of their own — they react to events and publish results.

### The order saga

An order touches three services, so instead of one big transaction each service
does its own step and announces the result:

```
POST /orders ──► order.created
                      │
                      ▼
             inventory reserves stock
                      │
           ┌──────────┴───────────┐
    inventory.reserved      inventory.failed
           │                      │
           ▼                      │
     payment charges              │
           │                      │
    ┌──────┴───────┐              │
payment.confirmed  payment.failed │
      │                 │         │
      ▼                 ▼         ▼
order.confirmed      order.cancelled
                            │
                            ▼
                inventory releases stock
```

There is nothing to roll back automatically, so every failure publishes
`order.cancelled` and inventory-service puts the stock back. Cancelling an
order from the API takes the same path.

Releases only touch reservations still marked `active`, so a duplicate
`order.cancelled` does nothing the second time. Kafka delivers at least once,
so this matters.

## Services

| Service | Port | Storage | Publishes | Consumes |
|---|---|---|---|---|
| api-gateway | 3000 | — | — | — |
| user-service | 8081 | Postgres | `user.events` | — |
| order-service | 8082 | Postgres | `order.events` | `inventory.events`, `payment.events` |
| inventory-service | 8083 | Postgres | `inventory.events` | `order.events` |
| payment-service | — | Postgres | `payment.events` | `inventory.events` |
| notification-service | — | Postgres | — | all four above |

All the services use the same JSON event shape, one flat struct with optional
fields. Each service declares only the fields it uses — the rest are dropped
when the JSON is decoded.

```json
{
  "eventType": "order.created",
  "orderId": "...",
  "userId": "...",
  "email": "...",
  "items": [{ "productId": "p1", "quantity": 2, "unitPrice": 29.99 }],
  "total": 59.98,
  "timestamp": "..."
}
```

## Running it

You need Docker.

```bash
make up      # builds the images and starts Postgres, Kafka and the six services
make seed    # seeds stock and places two orders
make logs    # follow the logs
make down    # stop everything
```

`make seed` places one order that goes through, and one over the mock payment
gateway's decline limit. The second prints the stock level before and after so
you can see it come back.

Each service creates its own tables on first start, and Kafka creates the
topics on first publish.

### API

| Method | Path | |
|---|---|---|
| POST | `/auth/register` | `{ email, password, name }` |
| POST | `/auth/login` | `{ email, password }` |
| GET | `/products` | |
| GET | `/products/:id/stock?quantity=n` | |
| POST | `/orders` | `{ items: [{ productId, quantity, unitPrice }] }`, needs a token |
| GET | `/orders/:id` | needs a token |
| DELETE | `/orders/:id` | cancel, needs a token |
| GET | `/health` | |

Authenticated routes take the JWT from login as `Authorization: Bearer <token>`.

```bash
TOKEN=$(curl -s -X POST localhost:3000/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"testuser@pulse.dev","password":"Test@123456"}' | jq -r .token)

curl -X POST localhost:3000/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"productId":"p1","quantity":2,"unitPrice":29.99}]}'
```

### Environment variables

`docker-compose.yml` sets these already. They only matter if you run a service
outside Docker.

| Variable | Default | Used by |
|---|---|---|
| `DATABASE_URL` | — | every Go service, required |
| `JWT_SECRET` | — | user-service, api-gateway, required |
| `KAFKA_BROKERS` | `localhost:9092` | every Go service |
| `PORT` | per service | user, order, inventory, gateway |
| `PAYMENT_DECLINE_OVER` | `5000` | payment-service |
| `USER_SERVICE_URL` etc. | `http://localhost:808x` | api-gateway |

## Tests

```bash
make test
```

Unit tests for the order total and validation, password hashing, stock
availability, the mock payment gateway, and the notification messages. The saga
itself is checked by running `make seed`.

## Kubernetes

Plain manifests in `k8s/`, enough to run the same thing on minikube or kind.

```bash
# Build the images into the cluster's docker daemon.
eval $(minikube docker-env)
for s in user-service order-service inventory-service \
         payment-service notification-service api-gateway; do
  docker build -t pulse/$s:latest services/$s
done

kubectl apply -f k8s/
kubectl get pods -n pulse
minikube service api-gateway -n pulse
```

Postgres uses an `emptyDir`, so the data goes away with the pod. The secret is
checked in, which is fine for a demo cluster but not for anything real.

## Layout

```
services/          one flat Go package per service
k8s/               Deployment and Service per component
scripts/           seed script that walks through the saga
docker-compose.yml everything needed to run it locally
```

## Built with

Go, TypeScript, Express, Postgres (pgx), Kafka (kafka-go), JWT, bcrypt, Docker
Compose, Kubernetes, GitHub Actions.

## Things I'd add next

- Retries and a dead letter topic for events that keep failing
- A real payment provider behind the `Gateway` interface
- Order history endpoint — right now you can only fetch an order by ID
- Integration tests that run the saga against a throwaway Kafka

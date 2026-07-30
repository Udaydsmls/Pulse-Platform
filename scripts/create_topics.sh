#!/usr/bin/env bash
# Creates the Kafka topics the services publish to.
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE="docker compose -f ${PROJECT_ROOT}/docker-compose.dev.yml"

TOPICS=(
  "user.events"
  "order.events"
  "inventory.events"
  "payment.events"
  "notification.events"
)

# Auto-creation is off in docker-compose.dev.yml, so topics are created here with
# a known partition count. Events are keyed by order ID, which keeps all events
# for one order on the same partition and therefore in order.
#
# kafka-topics ships inside the broker image, so it runs there rather than
# needing a Kafka install on the host.
for topic in "${TOPICS[@]}"; do
  echo "creating topic ${topic}"
  $COMPOSE exec -T kafka kafka-topics \
    --bootstrap-server localhost:9092 \
    --create --if-not-exists \
    --topic "${topic}" \
    --partitions 3 \
    --replication-factor 1
done

echo ""
echo "topics now on the broker:"
$COMPOSE exec -T kafka kafka-topics --bootstrap-server localhost:9092 --list

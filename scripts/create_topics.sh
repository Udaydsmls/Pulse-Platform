#!/usr/bin/env bash
set -euo pipefail

KAFKA_BOOTSTRAP_SERVERS="${KAFKA_BOOTSTRAP_SERVERS:-localhost:9092}"
PARTITIONS=3
REPLICATION_FACTOR=1

TOPICS=(
  "user.events"
  "order.events"
  "inventory.events"
  "payment.events"
  "notification.events"
)

echo "Using Kafka bootstrap servers: ${KAFKA_BOOTSTRAP_SERVERS}"

# Retrieve existing topics once
existing_topics=$(kafka-topics.sh --bootstrap-server "${KAFKA_BOOTSTRAP_SERVERS}" --list 2>/dev/null || true)

for topic in "${TOPICS[@]}"; do
  if echo "${existing_topics}" | grep -qx "${topic}"; then
    echo "[SKIP]   Topic '${topic}' already exists."
  else
    echo "[CREATE] Creating topic '${topic}' (partitions=${PARTITIONS}, replication-factor=${REPLICATION_FACTOR})..."
    kafka-topics.sh \
      --bootstrap-server "${KAFKA_BOOTSTRAP_SERVERS}" \
      --create \
      --topic "${topic}" \
      --partitions "${PARTITIONS}" \
      --replication-factor "${REPLICATION_FACTOR}"
    echo "[OK]     Topic '${topic}' created."
  fi
done

echo ""
echo "All Kafka topics are ready."
kafka-topics.sh --bootstrap-server "${KAFKA_BOOTSTRAP_SERVERS}" --list

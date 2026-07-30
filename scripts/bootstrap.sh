#!/usr/bin/env bash
# Starts local infrastructure, creates the Kafka topics and seeds stock.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE="docker compose -f ${PROJECT_ROOT}/docker-compose.dev.yml"

echo "==> starting postgres, kafka and redis"
$COMPOSE up -d

# The compose file defines a Kafka healthcheck, so wait on that instead of
# guessing how long the broker needs.
echo "==> waiting for kafka"
for attempt in $(seq 1 30); do
  if [ "$($COMPOSE ps kafka --format '{{.Health}}')" = "healthy" ]; then
    echo "    kafka is ready"
    break
  fi
  if [ "${attempt}" -eq 30 ]; then
    echo "ERROR: kafka did not become healthy. Check: ${COMPOSE} logs kafka" >&2
    exit 1
  fi
  sleep 5
done

echo ""
echo "==> creating kafka topics"
bash "${SCRIPT_DIR}/create_topics.sh"

echo ""
echo "Infrastructure is ready. Next:"
echo "  make run-all           start the services"
echo "  ./scripts/seed_data.sh seed stock and walk through the saga"

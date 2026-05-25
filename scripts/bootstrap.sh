#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

KAFKA_BOOTSTRAP_SERVERS="${KAFKA_BOOTSTRAP_SERVERS:-localhost:9092}"
KAFKA_READY_RETRIES=30
KAFKA_RETRY_SLEEP=5

# ── 1. Check prerequisites ───────────────────────────────────────────────────
echo "==> Checking prerequisites..."

if ! command -v docker &>/dev/null; then
  echo "ERROR: docker is not installed or not in PATH." >&2
  exit 1
fi

if ! docker compose version &>/dev/null; then
  echo "ERROR: docker compose (v2) is not available. Install the Docker Compose plugin." >&2
  exit 1
fi

echo "    docker:         $(docker --version)"
echo "    docker compose: $(docker compose version)"

# ── 2. Start services ────────────────────────────────────────────────────────
echo ""
echo "==> Starting dev services..."
docker compose -f "${PROJECT_ROOT}/docker-compose.dev.yml" up -d
echo "    Containers started."

# ── 3. Wait for Kafka ────────────────────────────────────────────────────────
echo ""
echo "==> Waiting for Kafka to be ready at ${KAFKA_BOOTSTRAP_SERVERS}..."

attempt=1
until kafka-topics.sh --bootstrap-server "${KAFKA_BOOTSTRAP_SERVERS}" --list &>/dev/null; do
  if [ "${attempt}" -gt "${KAFKA_READY_RETRIES}" ]; then
    echo "ERROR: Kafka did not become ready after $((KAFKA_READY_RETRIES * KAFKA_RETRY_SLEEP))s." >&2
    echo "Check logs with: docker compose -f docker-compose.dev.yml logs kafka" >&2
    exit 1
  fi
  echo "    Kafka not ready yet (attempt ${attempt}/${KAFKA_READY_RETRIES}). Retrying in ${KAFKA_RETRY_SLEEP}s..."
  sleep "${KAFKA_RETRY_SLEEP}"
  attempt=$((attempt + 1))
done

echo "    Kafka is ready."

# ── 4. Create Kafka topics ───────────────────────────────────────────────────
echo ""
echo "==> Creating Kafka topics..."
bash "${SCRIPT_DIR}/create_topics.sh"

# ── 5. Done ──────────────────────────────────────────────────────────────────
echo ""
echo "=========================================="
echo " Dev environment ready."
echo " API Gateway: http://localhost:3000"
echo "=========================================="

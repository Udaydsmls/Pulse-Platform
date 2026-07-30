#!/usr/bin/env bash
# Seeds stock, then walks an order through the saga twice: once succeeding and
# once failing, so you can watch the compensating rollback release the stock.
#
# Run after `make run-all`, once the services have created their tables.
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:3000}"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE="docker compose -f ${PROJECT_ROOT}/docker-compose.dev.yml"

EMAIL="testuser@pulse-platform.dev"
PASSWORD="Test@123456"

psql() {
  $COMPOSE exec -T postgres psql -U pulse -d pulsedb -tAc "$1"
}

stock_of() {
  psql "SELECT stock_level - reserved FROM stock_items WHERE product_id = '$1';"
}

# ── Stock ────────────────────────────────────────────────────────────────────
# Product IDs match the catalogue the gateway serves at GET /products.
echo "==> seeding stock"
psql "
  INSERT INTO stock_items (product_id, stock_level) VALUES
    ('p1', 150), ('p2', 75), ('p3', 200)
  ON CONFLICT (product_id) DO UPDATE SET stock_level = EXCLUDED.stock_level, reserved = 0;
"
psql "DELETE FROM reservations;"
echo "    p1=$(stock_of p1) p2=$(stock_of p2) p3=$(stock_of p3) available"

# ── Account ──────────────────────────────────────────────────────────────────
echo ""
echo "==> registering ${EMAIL} (ignored if it already exists)"
curl -s -o /dev/null -X POST "${GATEWAY_URL}/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Test User\",\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" || true

TOKEN=$(curl -s -X POST "${GATEWAY_URL}/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" |
  grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "${TOKEN}" ]; then
  echo "ERROR: could not log in. Is user-service running?" >&2
  exit 1
fi
echo "    logged in"

place_order() {
  curl -s -X POST "${GATEWAY_URL}/orders" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "{\"items\":$1}"
}

order_status() {
  psql "SELECT status FROM orders WHERE id = '$1';"
}

# ── Happy path ───────────────────────────────────────────────────────────────
echo ""
echo "==> happy path: 2x p1 at 29.99 = 59.98, under the decline limit"
ORDER_ID=$(place_order '[{"productId":"p1","quantity":2,"unitPrice":29.99}]' |
  grep -o '"orderId":"[^"]*"' | cut -d'"' -f4)
echo "    order ${ORDER_ID} created"

# The saga runs across three services over Kafka, so give it a moment.
sleep 5
echo "    status:      $(order_status "${ORDER_ID}")   (expected: confirmed)"
echo "    p1 available: $(stock_of p1)          (expected: 148, stock stays reserved)"

# ── Rollback path ────────────────────────────────────────────────────────────
echo ""
echo "==> rollback path: 100x p3 at 99.99 = 9999.00, over the decline limit"
echo "    stock is reserved first, then payment fails and it must be released"
BEFORE=$(stock_of p3)

FAIL_ORDER_ID=$(place_order '[{"productId":"p3","quantity":100,"unitPrice":99.99}]' |
  grep -o '"orderId":"[^"]*"' | cut -d'"' -f4)
echo "    order ${FAIL_ORDER_ID} created"

sleep 5
echo "    status:       $(order_status "${FAIL_ORDER_ID}")   (expected: cancelled)"
echo "    p3 available: $(stock_of p3) (was ${BEFORE} — the compensating event put it back)"

echo ""
echo "==> done. Credentials: ${EMAIL} / ${PASSWORD}"
echo "    Watch updates live:"
echo "      websocat 'ws://localhost:3000/ws/orders?token=${TOKEN:0:12}...'"

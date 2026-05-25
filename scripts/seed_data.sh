#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:3000}"

echo "==> Seeding development data against ${GATEWAY_URL}"
echo ""

# ── 1. Register a test user ──────────────────────────────────────────────────
echo "--- Step 1: Register test user ---"
REGISTER_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${GATEWAY_URL}/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "name":     "Test User",
    "email":    "testuser@pulse-platform.dev",
    "password": "Test@123456"
  }')

REGISTER_BODY=$(echo "${REGISTER_RESPONSE}" | head -n -1)
REGISTER_STATUS=$(echo "${REGISTER_RESPONSE}" | tail -n 1)

echo "    Status: ${REGISTER_STATUS}"
echo "    Body:   ${REGISTER_BODY}"

if [ "${REGISTER_STATUS}" != "201" ] && [ "${REGISTER_STATUS}" != "200" ]; then
  echo "    WARN: Unexpected status ${REGISTER_STATUS}. User may already exist — continuing to login."
fi

# ── 2. Login to get auth token ───────────────────────────────────────────────
echo ""
echo "--- Step 2: Login ---"
LOGIN_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${GATEWAY_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email":    "testuser@pulse-platform.dev",
    "password": "Test@123456"
  }')

LOGIN_BODY=$(echo "${LOGIN_RESPONSE}" | head -n -1)
LOGIN_STATUS=$(echo "${LOGIN_RESPONSE}" | tail -n 1)

echo "    Status: ${LOGIN_STATUS}"

if [ "${LOGIN_STATUS}" != "200" ]; then
  echo "ERROR: Login failed with status ${LOGIN_STATUS}. Body: ${LOGIN_BODY}" >&2
  exit 1
fi

# Extract token — works for {"token":"..."} or {"access_token":"..."}
AUTH_TOKEN=$(echo "${LOGIN_BODY}" | grep -o '"token"\s*:\s*"[^"]*"' | head -1 | sed 's/.*"\s*:\s*"\([^"]*\)"/\1/')
if [ -z "${AUTH_TOKEN}" ]; then
  AUTH_TOKEN=$(echo "${LOGIN_BODY}" | grep -o '"access_token"\s*:\s*"[^"]*"' | head -1 | sed 's/.*"\s*:\s*"\([^"]*\)"/\1/')
fi

if [ -z "${AUTH_TOKEN}" ]; then
  echo "ERROR: Could not extract auth token from login response: ${LOGIN_BODY}" >&2
  exit 1
fi

echo "    Token obtained: ${AUTH_TOKEN:0:20}..."

# ── 3. Create a test order ───────────────────────────────────────────────────
echo ""
echo "--- Step 3: Create test order ---"
ORDER_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${GATEWAY_URL}/orders" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -d '{
    "items": [
      { "productId": "prod-001", "quantity": 2, "unitPrice": 29.99 },
      { "productId": "prod-002", "quantity": 1, "unitPrice": 49.99 }
    ],
    "shippingAddress": {
      "street":  "123 Test Street",
      "city":    "Boston",
      "state":   "MA",
      "zip":     "02101",
      "country": "US"
    }
  }')

ORDER_BODY=$(echo "${ORDER_RESPONSE}" | head -n -1)
ORDER_STATUS=$(echo "${ORDER_RESPONSE}" | tail -n 1)

echo "    Status: ${ORDER_STATUS}"
echo "    Body:   ${ORDER_BODY}"

if [ "${ORDER_STATUS}" != "201" ] && [ "${ORDER_STATUS}" != "200" ]; then
  echo "ERROR: Order creation failed with status ${ORDER_STATUS}." >&2
  exit 1
fi

# ── 4. Done ──────────────────────────────────────────────────────────────────
echo ""
echo "==> Seed complete."
echo "    User:  testuser@pulse-platform.dev / Test@123456"
echo "    Token: ${AUTH_TOKEN:0:20}..."

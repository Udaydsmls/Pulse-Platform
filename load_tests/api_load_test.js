import http from "k6/http";
import { check, sleep } from "k6";

const BASE_URL = __ENV.BASE_URL || "http://localhost:3000";

export const options = {
  stages: [
    { duration: "1m", target: 100 },  // Ramp up to 100 VUs over 1 minute
    { duration: "3m", target: 500 },  // Hold at 500 VUs for 3 minutes
    { duration: "1m", target: 0 },    // Ramp down to 0 over 1 minute
  ],
  thresholds: {
    http_req_duration: ["p(95)<200"],  // 95th percentile latency < 200ms
    http_req_failed: ["rate<0.01"],    // Error rate < 1%
  },
};

// ── Setup: register and login a test user, return auth token ─────────────────
export function setup() {
  const registerPayload = JSON.stringify({
    name: "Load Test User",
    email: "loadtest@pulse-platform.dev",
    password: "LoadTest@123456",
  });

  const headers = { "Content-Type": "application/json" };

  // Register (ignore 409 if user already exists)
  http.post(`${BASE_URL}/auth/register`, registerPayload, { headers });

  // Login
  const loginRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({
      email: "loadtest@pulse-platform.dev",
      password: "LoadTest@123456",
    }),
    { headers }
  );

  check(loginRes, {
    "setup: login succeeded": (r) => r.status === 200,
  });

  const body = loginRes.json();
  const token = body.token || body.access_token;
  if (!token) {
    throw new Error(`setup: could not extract auth token. Response: ${loginRes.body}`);
  }

  return { token };
}

// ── Default function: mixed read/write workload ──────────────────────────────
export default function (data) {
  const { token } = data;
  const authHeaders = {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };

  // Weighted random: 0.0–0.29 → GET /products, 0.30–0.69 → POST /orders, 0.70–0.99 → GET /orders/:id
  const rand = Math.random();

  if (rand < 0.30) {
    // ── 30%: Browse products ───────────────────────────────────────────────
    const res = http.get(`${BASE_URL}/products`, { headers: authHeaders });
    check(res, {
      "GET /products status 200": (r) => r.status === 200,
    });

  } else if (rand < 0.70) {
    // ── 40%: Create an order ───────────────────────────────────────────────
    const productIds = ["prod-001", "prod-002", "prod-003", "prod-004", "prod-005"];
    const randomProduct = productIds[Math.floor(Math.random() * productIds.length)];
    const quantity = Math.floor(Math.random() * 3) + 1;

    const orderPayload = JSON.stringify({
      items: [
        {
          productId: randomProduct,
          quantity,
          unitPrice: parseFloat((Math.random() * 100 + 5).toFixed(2)),
        },
      ],
      shippingAddress: {
        street: "123 Load Test Ave",
        city: "Boston",
        state: "MA",
        zip: "02101",
        country: "US",
      },
    });

    const res = http.post(`${BASE_URL}/orders`, orderPayload, { headers: authHeaders });
    const created = check(res, {
      "POST /orders status 201": (r) => r.status === 201,
    });

    // Store created order ID for potential follow-up reads
    if (created) {
      const body = res.json();
      if (body && body.id) {
        // Immediately fetch the created order (simulates order confirmation page)
        const getRes = http.get(`${BASE_URL}/orders/${body.id}`, { headers: authHeaders });
        check(getRes, {
          "GET /orders/:id (post-create) status 200": (r) => r.status === 200,
        });
      }
    }

  } else {
    // ── 30%: Fetch a specific order (uses a known test order or a random ID) ─
    // In a real scenario you'd share order IDs from the setup phase. Here we
    // use a stable order ID seeded in the dev environment.
    const orderId = `order-${Math.floor(Math.random() * 100) + 1}`;
    const res = http.get(`${BASE_URL}/orders/${orderId}`, { headers: authHeaders });
    check(res, {
      "GET /orders/:id status 200 or 404": (r) => r.status === 200 || r.status === 404,
    });
  }

  sleep(0.5);
}

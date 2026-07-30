import http from "k6/http";
import { check, sleep } from "k6";

// Load test for the api-gateway. Run with:
//   k6 run load_tests/api_load_test.js
//   BASE_URL=https://api.pulse-platform.example.com k6 run load_tests/api_load_test.js
//
// Run ./scripts/seed_data.sh first so the products below have stock. Note that
// what is measured here is gateway latency: POST /orders returns as soon as
// order.created is published, and the rest of the saga runs asynchronously.

const BASE_URL = __ENV.BASE_URL || "http://localhost:3000";

const USER = {
  name: "Load Test User",
  email: "loadtest@pulse-platform.dev",
  password: "LoadTest@123456",
};

// Product IDs from the gateway's catalogue, kept cheap so the mock payment
// gateway approves them and stock lasts the run.
const PRODUCTS = ["p1", "p2", "p3"];

export const options = {
  stages: [
    { duration: "1m", target: 100 },
    { duration: "3m", target: 500 },
    { duration: "1m", target: 0 },
  ],
  thresholds: {
    "http_req_duration": ["p(95)<200"],
    "http_req_failed": ["rate<0.01"],
  },
};

const jsonHeaders = { "Content-Type": "application/json" };

export function setup() {
  // Already-registered is fine — the login below is what matters.
  http.post(`${BASE_URL}/auth/register`, JSON.stringify(USER), { headers: jsonHeaders });

  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: USER.email, password: USER.password }),
    { headers: jsonHeaders },
  );

  if (res.status !== 200) {
    throw new Error(`setup: login failed with ${res.status}: ${res.body}`);
  }
  return { token: res.json("token") };
}

export default function (data) {
  const headers = {
    ...jsonHeaders,
    Authorization: `Bearer ${data.token}`,
  };

  // Roughly 70% reads, 30% writes — a browsing-heavy shopping pattern.
  if (Math.random() < 0.7) {
    browseProducts(headers);
  } else {
    placeAndReadOrder(headers);
  }

  sleep(0.5);
}

function browseProducts(headers) {
  const res = http.get(`${BASE_URL}/products`, { headers });
  check(res, { "GET /products is 200": (r) => r.status === 200 });
}

function placeAndReadOrder(headers) {
  const body = JSON.stringify({
    items: [
      {
        productId: PRODUCTS[Math.floor(Math.random() * PRODUCTS.length)],
        quantity: 1,
        unitPrice: 29.99,
      },
    ],
  });

  const res = http.post(`${BASE_URL}/orders`, body, { headers });
  const created = check(res, { "POST /orders is 201": (r) => r.status === 201 });
  if (!created) {
    return;
  }

  // Read the order straight back, the way a confirmation page would.
  const orderId = res.json("orderId");
  const readRes = http.get(`${BASE_URL}/orders/${orderId}`, { headers });
  check(readRes, { "GET /orders/:id is 200": (r) => r.status === 200 });
}

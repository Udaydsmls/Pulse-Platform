// Configuration read from the environment. JWT_SECRET is required so the
// gateway fails at startup instead of verifying tokens against an empty key.

function required(key: string): string {
  const value = process.env[key];
  if (!value) {
    throw new Error(`Missing required environment variable: ${key}`);
  }
  return value;
}

function optional(key: string, fallback: string): string {
  return process.env[key] || fallback;
}

export const config = {
  port: Number(optional('PORT', '3000')),
  jwtSecret: required('JWT_SECRET'),

  // Ports match the defaults in each Go service.
  userServiceUrl: optional('USER_SERVICE_URL', 'http://localhost:8081'),
  orderServiceUrl: optional('ORDER_SERVICE_URL', 'http://localhost:8082'),
  inventoryServiceUrl: optional('INVENTORY_SERVICE_URL', 'http://localhost:8083'),
};

/**
 * Configuration read from the environment.
 *
 * Secrets and connection strings are required, so a misconfigured gateway fails
 * at startup instead of, say, verifying JWTs against an empty signing key.
 */

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
  redisUrl: required('REDIS_URL'),

  // Ports match the defaults in each Go service's loadConfig().
  userServiceAddr: optional('GRPC_USER_SERVICE_ADDR', 'localhost:50051'),
  orderServiceAddr: optional('GRPC_ORDER_SERVICE_ADDR', 'localhost:50052'),
  inventoryServiceAddr: optional('GRPC_INVENTORY_SERVICE_ADDR', 'localhost:50053'),

  kafkaBrokers: optional('KAFKA_BROKERS', 'localhost:9092').split(','),
  corsOrigins: optional('CORS_ORIGINS', 'http://localhost:3000').split(','),
};

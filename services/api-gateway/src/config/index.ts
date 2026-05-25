/**
 * Application configuration loaded from environment variables.
 * All fields are validated at startup; missing required fields throw.
 */

export interface Config {
  PORT: number;
  REDIS_URL: string;
  GRPC_USER_SERVICE_ADDR: string;
  GRPC_ORDER_SERVICE_ADDR: string;
  GRPC_INVENTORY_SERVICE_ADDR: string;
  KAFKA_BROKERS: string[];
  ELASTICSEARCH_URL: string;
  JWT_SECRET: string;
  CORS_ORIGINS: string[];
}

function requireEnv(key: string): string {
  const value = process.env[key];
  if (!value) {
    throw new Error(`Missing required environment variable: ${key}`);
  }
  return value;
}

/**
 * Reads and validates configuration from process.env.
 * Throws if any required variable is absent.
 */
export function loadConfig(): Config {
  return {
    PORT: parseInt(process.env['PORT'] ?? '3000', 10),
    REDIS_URL: requireEnv('REDIS_URL'),
    GRPC_USER_SERVICE_ADDR: process.env['GRPC_USER_SERVICE_ADDR'] ?? 'localhost:50051',
    GRPC_ORDER_SERVICE_ADDR: process.env['GRPC_ORDER_SERVICE_ADDR'] ?? 'localhost:50052',
    GRPC_INVENTORY_SERVICE_ADDR: process.env['GRPC_INVENTORY_SERVICE_ADDR'] ?? 'localhost:50054',
    KAFKA_BROKERS: (process.env['KAFKA_BROKERS'] ?? 'localhost:9092')
      .split(',')
      .map((b) => b.trim()),
    ELASTICSEARCH_URL: requireEnv('ELASTICSEARCH_URL'),
    JWT_SECRET: requireEnv('JWT_SECRET'),
    CORS_ORIGINS: (process.env['CORS_ORIGINS'] ?? 'http://localhost:3000')
      .split(',')
      .map((o) => o.trim()),
  };
}

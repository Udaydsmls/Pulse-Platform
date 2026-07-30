/**
 * config.ts reads the environment when the module is first imported, so each
 * test sets up process.env and then re-imports it.
 */

const requiredVars = {
  JWT_SECRET: 'test-secret',
  REDIS_URL: 'redis://localhost:6379',
};

function loadConfig() {
  return require('./config').config as typeof import('./config').config;
}

describe('config', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv, ...requiredVars };
  });

  afterAll(() => {
    process.env = originalEnv;
  });

  it('throws when a required variable is missing', () => {
    delete process.env.JWT_SECRET;
    expect(loadConfig).toThrow('Missing required environment variable: JWT_SECRET');
  });

  it('defaults the service addresses to the ports the Go services listen on', () => {
    const config = loadConfig();
    expect(config.userServiceAddr).toBe('localhost:50051');
    expect(config.orderServiceAddr).toBe('localhost:50052');
    expect(config.inventoryServiceAddr).toBe('localhost:50053');
  });

  it('splits comma-separated lists', () => {
    process.env.KAFKA_BROKERS = 'a:9092,b:9092';
    expect(loadConfig().kafkaBrokers).toEqual(['a:9092', 'b:9092']);
  });
});

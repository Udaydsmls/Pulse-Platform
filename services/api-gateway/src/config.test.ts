// config.ts reads the environment when it is first imported, so each test sets
// up process.env and then re-imports it.

function loadConfig() {
  return require('./config').config as typeof import('./config').config;
}

describe('config', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv, JWT_SECRET: 'test-secret' };
  });

  afterAll(() => {
    process.env = originalEnv;
  });

  it('throws when JWT_SECRET is missing', () => {
    delete process.env.JWT_SECRET;
    expect(loadConfig).toThrow('Missing required environment variable: JWT_SECRET');
  });

  it('defaults the service URLs to the ports the Go services listen on', () => {
    const config = loadConfig();
    expect(config.userServiceUrl).toBe('http://localhost:8081');
    expect(config.orderServiceUrl).toBe('http://localhost:8082');
    expect(config.inventoryServiceUrl).toBe('http://localhost:8083');
  });

  it('reads the service URLs from the environment', () => {
    process.env.USER_SERVICE_URL = 'http://user-service:8081';
    expect(loadConfig().userServiceUrl).toBe('http://user-service:8081');
  });
});

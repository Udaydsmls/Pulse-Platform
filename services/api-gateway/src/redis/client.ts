import Redis from 'ioredis';
import { loadConfig } from '../config';

const config = loadConfig();

/**
 * Primary Redis client used for caching, rate limiting, and JWT blacklisting.
 */
export const redisClient = new Redis(config.REDIS_URL, {
  lazyConnect: true,
  enableReadyCheck: true,
  maxRetriesPerRequest: 3,
});

/**
 * Dedicated Redis connection reserved for pub/sub subscriptions.
 * A subscribed client cannot send regular commands on the same connection.
 */
export const subscriber = new Redis(config.REDIS_URL, {
  lazyConnect: true,
  enableReadyCheck: true,
  maxRetriesPerRequest: 3,
});

const BLACKLIST_PREFIX = 'jwt:blacklist:';
const CACHE_PREFIX = 'cache:';
const ORDER_CHANNEL_PREFIX = 'order_updates:';

/**
 * Adds a JWT to the blacklist with the given TTL in seconds.
 */
export async function blacklistToken(token: string, ttlSeconds: number): Promise<void> {
  await redisClient.setex(`${BLACKLIST_PREFIX}${token}`, ttlSeconds, '1');
}

/**
 * Returns true if the given JWT has been blacklisted.
 */
export async function isTokenBlacklisted(token: string): Promise<boolean> {
  const result = await redisClient.get(`${BLACKLIST_PREFIX}${token}`);
  return result !== null;
}

/**
 * Retrieves a cached value by key. Returns null on cache miss.
 */
export async function cacheGet(key: string): Promise<string | null> {
  return redisClient.get(`${CACHE_PREFIX}${key}`);
}

/**
 * Stores a value in the cache with the given TTL in seconds.
 */
export async function cacheSet(key: string, value: string, ttlSeconds: number): Promise<void> {
  await redisClient.setex(`${CACHE_PREFIX}${key}`, ttlSeconds, value);
}

/**
 * Publishes an order update event to the user-specific Redis pub/sub channel.
 */
export async function publishOrderUpdate(userId: string, data: unknown): Promise<void> {
  const channel = `${ORDER_CHANNEL_PREFIX}${userId}`;
  await redisClient.publish(channel, JSON.stringify(data));
}

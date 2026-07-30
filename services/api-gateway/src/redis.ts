import Redis from 'ioredis';
import { config } from './config';

/** Commands: caching and the JWT denylist. */
export const redis = new Redis(config.redisUrl, { lazyConnect: true });

/**
 * A Redis connection that is subscribed to a channel cannot issue normal
 * commands, so publishing needs its own connection.
 */
export const publisher = new Redis(config.redisUrl, { lazyConnect: true });

/** Channel a user's browser listens on for order updates. */
export function orderChannel(userId: string): string {
  return `order_updates:${userId}`;
}

/** Marks a token as logged out until it would have expired anyway. */
export async function denyToken(token: string, ttlSeconds: number): Promise<void> {
  await redis.setex(`jwt:denied:${token}`, ttlSeconds, '1');
}

export async function isTokenDenied(token: string): Promise<boolean> {
  return (await redis.get(`jwt:denied:${token}`)) !== null;
}

export async function cacheGet(key: string): Promise<string | null> {
  return redis.get(`cache:${key}`);
}

export async function cacheSet(key: string, value: string, ttlSeconds: number): Promise<void> {
  await redis.setex(`cache:${key}`, ttlSeconds, value);
}

/** Pushes an order update to whichever gateway holds that user's WebSocket. */
export async function publishOrderUpdate(userId: string, data: unknown): Promise<void> {
  await publisher.publish(orderChannel(userId), JSON.stringify(data));
}

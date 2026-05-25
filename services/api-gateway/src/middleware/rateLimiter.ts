import type { Request, Response, NextFunction } from 'express';
import { redisClient } from '../redis/client';

const WINDOW_MS = 60_000;
const AUTH_LIMIT = 100;
const ANON_LIMIT = 20;
const STRICT_LIMIT = 10;

/**
 * Sliding-window rate limiter backed by a Redis sorted set.
 * Returns true when the request should be allowed, false when the limit is exceeded.
 */
async function slidingWindowCheck(
  key: string,
  limit: number,
  windowMs: number,
): Promise<boolean> {
  const now = Date.now();
  const windowStart = now - windowMs;

  const pipeline = redisClient.pipeline();
  pipeline.zremrangebyscore(key, '-inf', windowStart);
  pipeline.zadd(key, now, `${now}-${Math.random()}`);
  pipeline.zcard(key);
  pipeline.pexpire(key, windowMs);

  const results = await pipeline.exec();

  if (!results) {
    return true;
  }

  const cardResult = results[2];
  if (!cardResult || cardResult[0]) {
    return true;
  }

  const count = cardResult[1] as number;
  return count <= limit;
}

function buildRateLimiterMiddleware(limit: number) {
  return async function rateLimiterMiddleware(
    req: Request,
    res: Response,
    next: NextFunction,
  ): Promise<void> {
    const identifier = req.user?.userId ?? req.ip ?? 'unknown';
    const effectiveLimit = req.user ? limit : Math.min(limit, ANON_LIMIT);
    const key = `rl:${effectiveLimit}:${identifier}`;

    const allowed = await slidingWindowCheck(key, effectiveLimit, WINDOW_MS);

    if (!allowed) {
      res.status(429).json({ error: 'Too Many Requests' });
      return;
    }

    next();
  };
}

/**
 * Standard sliding-window rate limiter.
 * Allows 100 req/min for authenticated users and 20 req/min for anonymous clients (by IP).
 */
export const rateLimiter = buildRateLimiterMiddleware(AUTH_LIMIT);

/**
 * Strict rate limiter for sensitive endpoints such as authentication.
 * Allows 10 req/min regardless of authentication status.
 */
export const strictRateLimiter = buildRateLimiterMiddleware(STRICT_LIMIT);

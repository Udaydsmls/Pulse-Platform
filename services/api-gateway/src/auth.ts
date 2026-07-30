import type { Request, Response, NextFunction } from 'express';
import jwt from 'jsonwebtoken';
import { config } from './config';
import { isTokenDenied } from './redis';

export interface AuthUser {
  userId: string;
  email: string;
}

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      user?: AuthUser;
      token?: string;
    }
  }
}

/**
 * Verifies the Bearer token and returns its claims, or null if the token is
 * missing, invalid, expired, or logged out.
 */
async function verify(req: Request): Promise<AuthUser | null> {
  const header = req.headers.authorization;
  if (!header?.startsWith('Bearer ')) {
    return null;
  }

  const token = header.slice('Bearer '.length);

  try {
    if (await isTokenDenied(token)) {
      return null;
    }

    // jwt.verify checks the signature and the exp claim, and throws on either.
    const claims = jwt.verify(token, config.jwtSecret) as AuthUser;
    req.token = token;
    return claims;
  } catch {
    return null;
  }
}

/** Rejects the request with 401 unless it carries a valid token. */
export async function requireAuth(req: Request, res: Response, next: NextFunction): Promise<void> {
  const user = await verify(req);
  if (!user) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }
  req.user = user;
  next();
}

/**
 * Attaches req.user when a valid token is present but lets the request through
 * either way. GraphQL uses this so a single endpoint can serve both public and
 * authenticated queries.
 */
export async function optionalAuth(req: Request, _res: Response, next: NextFunction): Promise<void> {
  const user = await verify(req);
  if (user) {
    req.user = user;
  }
  next();
}

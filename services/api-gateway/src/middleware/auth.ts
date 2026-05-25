import type { Request, Response, NextFunction } from 'express';
import jwt from 'jsonwebtoken';
import { loadConfig } from '../config';
import { isTokenBlacklisted } from '../redis/client';

export interface JwtPayload {
  userId: string;
  email: string;
  iat?: number;
  exp?: number;
}

declare global {
  namespace Express {
    interface Request {
      user?: JwtPayload;
      rawToken?: string;
    }
  }
}

const config = loadConfig();

async function verifyBearer(
  req: Request,
): Promise<JwtPayload | null> {
  const authHeader = req.headers['authorization'];
  if (!authHeader?.startsWith('Bearer ')) {
    return null;
  }

  const token = authHeader.slice(7);

  try {
    const blacklisted = await isTokenBlacklisted(token);
    if (blacklisted) {
      return null;
    }

    const payload = jwt.verify(token, config.JWT_SECRET) as JwtPayload;
    req.rawToken = token;
    return payload;
  } catch {
    return null;
  }
}

/**
 * Express middleware that requires a valid, non-blacklisted Bearer JWT.
 * Attaches the decoded payload to req.user.
 * Returns 401 if the token is missing, invalid, or blacklisted.
 */
export async function authenticateJWT(
  req: Request,
  res: Response,
  next: NextFunction,
): Promise<void> {
  const payload = await verifyBearer(req);
  if (!payload) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }
  req.user = payload;
  next();
}

/**
 * Express middleware that optionally attaches a verified JWT payload to req.user.
 * Does not reject the request if the token is missing or invalid.
 */
export async function optionalJWT(
  req: Request,
  _res: Response,
  next: NextFunction,
): Promise<void> {
  const payload = await verifyBearer(req);
  if (payload) {
    req.user = payload;
  }
  next();
}

import type { Request, Response, NextFunction } from 'express';
import jwt from 'jsonwebtoken';
import { config } from './config';

export interface AuthUser {
  userId: string;
  email: string;
}

declare global {
  namespace Express {
    interface Request {
      user?: AuthUser;
    }
  }
}

/** Rejects the request with 401 unless it carries a valid Bearer token. */
export function requireAuth(req: Request, res: Response, next: NextFunction): void {
  const header = req.headers.authorization;
  if (!header?.startsWith('Bearer ')) {
    res.status(401).json({ error: 'Unauthorized' });
    return;
  }

  try {
    // jwt.verify checks the signature and the expiry, and throws on either.
    req.user = jwt.verify(header.slice('Bearer '.length), config.jwtSecret) as AuthUser;
    next();
  } catch {
    res.status(401).json({ error: 'Unauthorized' });
  }
}

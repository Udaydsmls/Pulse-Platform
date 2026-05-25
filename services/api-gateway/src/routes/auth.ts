import { Router } from 'express';
import { z } from 'zod';
import jwt from 'jsonwebtoken';
import { strictRateLimiter } from '../middleware/rateLimiter';
import { authenticateJWT } from '../middleware/auth';
import { validateBody, validateQuery } from '../middleware/validate';
import { registerUser, loginUser, oauthLogin } from '../grpc/clients';
import { blacklistToken } from '../redis/client';
import { loadConfig } from '../config';

const config = loadConfig();

const registerSchema = z.object({
  email: z.string().email(),
  password: z.string().min(8),
  name: z.string().min(1).max(100),
});

const loginSchema = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

const oauthCallbackSchema = z.object({
  code: z.string().min(1),
});

export const authRouter = Router();

authRouter.use(strictRateLimiter);

authRouter.post(
  '/register',
  validateBody(registerSchema),
  async (req, res, next) => {
    try {
      const { email, password, name } = req.body as z.infer<typeof registerSchema>;
      const response = await registerUser({ email, password, name });
      res.status(201).json({ userId: response.userId, token: response.token });
    } catch (err) {
      next(err);
    }
  },
);

authRouter.post(
  '/login',
  validateBody(loginSchema),
  async (req, res, next) => {
    try {
      const { email, password } = req.body as z.infer<typeof loginSchema>;
      const response = await loginUser({ email, password });
      res.json({ token: response.token, userId: response.userId });
    } catch (err) {
      next(err);
    }
  },
);

authRouter.post(
  '/logout',
  authenticateJWT,
  async (req, res, next) => {
    try {
      const token = req.rawToken as string;

      let ttl = 3600;
      try {
        const decoded = jwt.decode(token) as { exp?: number } | null;
        if (decoded?.exp) {
          ttl = Math.max(decoded.exp - Math.floor(Date.now() / 1000), 1);
        }
      } catch {
        // use default ttl
      }

      await blacklistToken(token, ttl);
      res.status(200).json({ message: 'Logged out successfully' });
    } catch (err) {
      next(err);
    }
  },
);

authRouter.get(
  '/google/callback',
  validateQuery(oauthCallbackSchema),
  async (req, res, next) => {
    try {
      const { code } = req.query as z.infer<typeof oauthCallbackSchema>;
      const response = await oauthLogin({ code, provider: 'google' });
      res.json({ token: response.token, userId: response.userId });
    } catch (err) {
      next(err);
    }
  },
);

authRouter.get(
  '/github/callback',
  validateQuery(oauthCallbackSchema),
  async (req, res, next) => {
    try {
      const { code } = req.query as z.infer<typeof oauthCallbackSchema>;
      const response = await oauthLogin({ code, provider: 'github' });
      res.json({ token: response.token, userId: response.userId });
    } catch (err) {
      next(err);
    }
  },
);

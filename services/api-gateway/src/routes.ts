import { Router } from 'express';
import rateLimit from 'express-rate-limit';
import jwt from 'jsonwebtoken';
import { z } from 'zod';
import { requireAuth } from './auth';
import { denyToken, cacheGet, cacheSet } from './redis';
import { register, login, createOrder, cancelOrder, getOrder, checkStock } from './grpc';

/**
 * REST routes. Errors are passed to next() and handled by the error middleware
 * in index.ts, so gRPC failures don't leak internals to the client.
 */

const registerBody = z.object({
  email: z.string().email(),
  password: z.string().min(8),
  name: z.string().min(1).max(100),
});

const loginBody = z.object({
  email: z.string().email(),
  password: z.string().min(1),
});

const createOrderBody = z.object({
  items: z
    .array(
      z.object({
        productId: z.string().min(1),
        quantity: z.number().int().positive(),
        unitPrice: z.number().nonnegative(),
      }),
    )
    .min(1),
});

// ── /auth ────────────────────────────────────────────────────────────────────

export const authRouter = Router();

// Login and register are brute-force targets, so they get a tighter limit than
// the rest of the API.
authRouter.use(rateLimit({ windowMs: 60_000, max: 10 }));

authRouter.post('/register', async (req, res, next) => {
  try {
    res.status(201).json(await register(registerBody.parse(req.body)));
  } catch (err) {
    next(err);
  }
});

authRouter.post('/login', async (req, res, next) => {
  try {
    res.json(await login(loginBody.parse(req.body)));
  } catch (err) {
    next(err);
  }
});

authRouter.post('/logout', requireAuth, async (req, res, next) => {
  try {
    // JWTs can't be revoked, so the token goes on a denylist until its own
    // expiry — no point keeping it longer than that.
    const claims = jwt.decode(req.token!) as { exp?: number } | null;
    const secondsLeft = claims?.exp ? claims.exp - Math.floor(Date.now() / 1000) : 3600;

    await denyToken(req.token!, Math.max(secondsLeft, 1));
    res.json({ message: 'Logged out' });
  } catch (err) {
    next(err);
  }
});

// ── /orders ──────────────────────────────────────────────────────────────────

export const ordersRouter = Router();

ordersRouter.use(requireAuth);

ordersRouter.post('/', async (req, res, next) => {
  try {
    const { items } = createOrderBody.parse(req.body);
    const order = await createOrder({
      userId: req.user!.userId,
      email: req.user!.email,
      items,
    });
    res.status(201).json(order);
  } catch (err) {
    next(err);
  }
});

ordersRouter.get('/:orderId', async (req, res, next) => {
  try {
    res.json(await getOrder({ orderId: req.params.orderId }));
  } catch (err) {
    next(err);
  }
});

ordersRouter.delete('/:orderId', async (req, res, next) => {
  try {
    res.json(await cancelOrder({ orderId: req.params.orderId, userId: req.user!.userId }));
  } catch (err) {
    next(err);
  }
});

// ── /products ────────────────────────────────────────────────────────────────

export const productsRouter = Router();

// Stands in for a product catalogue service.
const catalogue = [
  { productId: 'p1', name: 'Widget Alpha', price: 29.99 },
  { productId: 'p2', name: 'Gadget Beta', price: 49.99 },
  { productId: 'p3', name: 'Device Gamma', price: 99.99 },
];

productsRouter.get('/', async (_req, res, next) => {
  try {
    const cached = await cacheGet('products');
    if (cached) {
      res.json(JSON.parse(cached));
      return;
    }

    await cacheSet('products', JSON.stringify(catalogue), 60);
    res.json(catalogue);
  } catch (err) {
    next(err);
  }
});

productsRouter.get('/:productId/stock', async (req, res, next) => {
  try {
    const quantity = Number(req.query.quantity ?? 1);
    res.json(await checkStock({ productId: req.params.productId, quantity }));
  } catch (err) {
    next(err);
  }
});

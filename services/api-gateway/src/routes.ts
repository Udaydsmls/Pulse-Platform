import { Router } from 'express';
import { z } from 'zod';
import { requireAuth } from './auth';
import { register, login, createOrder, cancelOrder, getOrder, checkStock } from './services';

// Errors are passed to next() and handled by the error middleware in index.ts.

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

export const authRouter = Router();

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
    res.json(await getOrder(req.params.orderId));
  } catch (err) {
    next(err);
  }
});

ordersRouter.delete('/:orderId', async (req, res, next) => {
  try {
    res.json(await cancelOrder(req.params.orderId, req.user!.userId));
  } catch (err) {
    next(err);
  }
});

export const productsRouter = Router();

// Hard-coded stand-in for a product catalogue.
const catalogue = [
  { productId: 'p1', name: 'Widget Alpha', price: 29.99 },
  { productId: 'p2', name: 'Gadget Beta', price: 49.99 },
  { productId: 'p3', name: 'Device Gamma', price: 99.99 },
];

productsRouter.get('/', (_req, res) => {
  res.json(catalogue);
});

productsRouter.get('/:productId/stock', async (req, res, next) => {
  try {
    const quantity = Number(req.query.quantity ?? 1);
    res.json(await checkStock(req.params.productId, quantity));
  } catch (err) {
    next(err);
  }
});

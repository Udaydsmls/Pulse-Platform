import { Router } from 'express';
import { z } from 'zod';
import { authenticateJWT } from '../middleware/auth';
import { rateLimiter } from '../middleware/rateLimiter';
import { validateBody, validateParams } from '../middleware/validate';
import { createOrder, getOrder, cancelOrder } from '../grpc/clients';

const orderItemSchema = z.object({
  productId: z.string().min(1),
  quantity: z.number().int().positive(),
  unitPrice: z.number().positive(),
});

const createOrderSchema = z.object({
  items: z.array(orderItemSchema).min(1),
});

const orderIdParamSchema = z.object({
  orderId: z.string().min(1),
});

export const ordersRouter = Router();

ordersRouter.use(authenticateJWT, rateLimiter);

ordersRouter.post(
  '/',
  validateBody(createOrderSchema),
  async (req, res, next) => {
    try {
      const { items } = req.body as z.infer<typeof createOrderSchema>;
      const userId = req.user!.userId;

      const response = await createOrder({
        userId,
        tenantId: userId,
        items: items.map((item) => ({
          productId: item.productId,
          quantity: item.quantity,
          unitPrice: item.unitPrice,
        })),
      });

      res.status(201).json({ orderId: response.orderId, status: response.status });
    } catch (err) {
      next(err);
    }
  },
);

ordersRouter.get(
  '/:orderId',
  validateParams(orderIdParamSchema),
  async (req, res, next) => {
    try {
      const { orderId } = req.params as z.infer<typeof orderIdParamSchema>;
      const order = await getOrder({ orderId });
      res.json(order);
    } catch (err) {
      next(err);
    }
  },
);

ordersRouter.delete(
  '/:orderId',
  validateParams(orderIdParamSchema),
  async (req, res, next) => {
    try {
      const { orderId } = req.params as z.infer<typeof orderIdParamSchema>;
      const userId = req.user!.userId;
      const response = await cancelOrder({ orderId, userId });
      res.json({ success: response.success });
    } catch (err) {
      next(err);
    }
  },
);

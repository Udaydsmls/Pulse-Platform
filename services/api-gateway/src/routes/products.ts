import { Router } from 'express';
import { z } from 'zod';
import { validateQuery } from '../middleware/validate';
import { cacheGet, cacheSet } from '../redis/client';
import { searchProducts } from '../search/client';

const PRODUCTS_CACHE_KEY = 'products:list';
const PRODUCTS_CACHE_TTL = 60;

const searchQuerySchema = z.object({
  q: z.string().min(1).max(200),
});

const mockProducts = [
  { productId: 'p1', name: 'Widget Alpha', description: 'A high-quality widget', price: 29.99, stock: 150 },
  { productId: 'p2', name: 'Gadget Beta', description: 'An essential gadget', price: 49.99, stock: 75 },
  { productId: 'p3', name: 'Device Gamma', description: 'Next-gen device', price: 99.99, stock: 30 },
];

/**
 * Router for product listing at /products.
 * GET / — returns product list, cached in Redis for 60 s.
 */
export const productsRouter = Router();

productsRouter.get('/', async (_req, res, next) => {
  try {
    const cached = await cacheGet(PRODUCTS_CACHE_KEY);
    if (cached) {
      res.json(JSON.parse(cached));
      return;
    }

    await cacheSet(PRODUCTS_CACHE_KEY, JSON.stringify(mockProducts), PRODUCTS_CACHE_TTL);
    res.json(mockProducts);
  } catch (err) {
    next(err);
  }
});

/**
 * Router for search at /search.
 * GET /products?q=... — full-text search via Elasticsearch.
 */
export const searchRouter = Router();

searchRouter.get(
  '/products',
  validateQuery(searchQuerySchema),
  async (req, res, next) => {
    try {
      const { q } = req.query as z.infer<typeof searchQuerySchema>;
      const results = await searchProducts(q);
      res.json(results);
    } catch (err) {
      next(err);
    }
  },
);

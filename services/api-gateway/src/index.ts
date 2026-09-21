import express from 'express';
import cors from 'cors';
import { ZodError } from 'zod';

import { config } from './config';
import { authRouter, ordersRouter, productsRouter } from './routes';
import { ServiceError } from './services';

const app = express();

app.use(cors());
app.use(express.json());

app.get('/health', (_req, res) => res.json({ status: 'ok' }));

app.use('/auth', authRouter);
app.use('/orders', ordersRouter);
app.use('/products', productsRouter);

// Bad input is the client's fault and a service error already has a status.
// Anything else is ours, so it is logged rather than returned.
app.use(
  (err: Error, _req: express.Request, res: express.Response, _next: express.NextFunction): void => {
    if (err instanceof ZodError) {
      res.status(400).json({ error: 'Invalid request', details: err.issues });
      return;
    }
    if (err instanceof ServiceError) {
      res.status(err.status).json({ error: err.message });
      return;
    }
    console.error('Unhandled error:', err);
    res.status(500).json({ error: 'Internal Server Error' });
  },
);

app.listen(config.port, () => {
  console.log(`api-gateway listening on :${config.port}`);
});

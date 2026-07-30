// OTel has to be set up before anything it instruments is imported, so this
// block stays at the very top of the file.
import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

const otel = new NodeSDK({ instrumentations: [getNodeAutoInstrumentations()] });
otel.start();

import http from 'http';
import express from 'express';
import helmet from 'helmet';
import cors from 'cors';
import rateLimit from 'express-rate-limit';
import { Kafka } from 'kafkajs';
import { ZodError } from 'zod';

import { config } from './config';
import { redis, publisher, publishOrderUpdate } from './redis';
import { optionalAuth } from './auth';
import { authRouter, ordersRouter, productsRouter } from './routes';
import { setupGraphQL } from './graphql';
import { setupWebSocket } from './websocket';

async function main(): Promise<void> {
  const app = express();

  app.use(helmet());
  app.use(cors({ origin: config.corsOrigins, credentials: true }));
  app.use(express.json({ limit: '1mb' }));
  app.use(rateLimit({ windowMs: 60_000, max: 200 }));

  // Runs before the routers so both REST and GraphQL see req.user.
  app.use(optionalAuth);

  app.get('/health', (_req, res) => res.json({ status: 'ok' }));

  app.use('/auth', authRouter);
  app.use('/orders', ordersRouter);
  app.use('/products', productsRouter);

  await setupGraphQL(app);

  // Bad input is the client's fault; everything else is ours and is logged
  // rather than returned, so internal errors don't leak to callers.
  app.use((err: Error, _req: express.Request, res: express.Response, _next: express.NextFunction) => {
    if (err instanceof ZodError) {
      res.status(400).json({ error: 'Invalid request', details: err.issues });
      return;
    }
    console.error('Unhandled error:', err);
    res.status(500).json({ error: 'Internal Server Error' });
  });

  const server = http.createServer(app);
  setupWebSocket(server);

  await redis.connect();
  await publisher.connect();

  const consumer = await startOrderUpdateRelay();

  server.listen(config.port, () => {
    console.log(`api-gateway listening on :${config.port}`);
  });

  const shutdown = async (): Promise<void> => {
    console.log('shutting down');
    await consumer.disconnect();
    await redis.quit();
    await publisher.quit();
    server.close(() => {
      void otel.shutdown().finally(() => process.exit(0));
    });
    // Don't wait forever for in-flight connections to drain.
    setTimeout(() => process.exit(1), 10_000).unref();
  };

  process.on('SIGTERM', () => void shutdown());
  process.on('SIGINT', () => void shutdown());
}

/**
 * Bridges Kafka to Redis pub/sub: notification-service publishes an update, and
 * we hand it to whichever gateway replica holds that user's WebSocket.
 */
async function startOrderUpdateRelay() {
  const kafka = new Kafka({ clientId: 'api-gateway', brokers: config.kafkaBrokers });
  const consumer = kafka.consumer({ groupId: 'api-gateway-ws-relay' });

  await consumer.connect();
  await consumer.subscribe({ topic: 'notification.events', fromBeginning: false });

  await consumer.run({
    eachMessage: async ({ message }) => {
      if (!message.value) return;

      try {
        const event = JSON.parse(message.value.toString()) as { user_id?: string };
        if (event.user_id) {
          await publishOrderUpdate(event.user_id, event);
        }
      } catch (err) {
        console.warn('Skipping malformed notification event:', err);
      }
    },
  });

  console.log('relaying notification.events to Redis pub/sub');
  return consumer;
}

main().catch((err) => {
  console.error('Fatal startup error:', err);
  process.exit(1);
});

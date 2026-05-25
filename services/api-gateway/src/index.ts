// OTel must be initialized before any other imports that instrument libraries.
import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

const otelSdk = new NodeSDK({
  instrumentations: [getNodeAutoInstrumentations()],
});
otelSdk.start();

import http from 'http';
import express from 'express';
import helmet from 'helmet';
import cors from 'cors';
import rateLimit from 'express-rate-limit';
import { Kafka } from 'kafkajs';
import pino from 'pino';

import { loadConfig } from './config';
import { redisClient, subscriber, publishOrderUpdate } from './redis/client';
import { authRouter } from './routes/auth';
import { ordersRouter } from './routes/orders';
import { productsRouter, searchRouter } from './routes/products';
import { setupWebSocket } from './websocket/handler';
import { setupGraphQL } from '../graphql/resolvers/index';
import { optionalJWT } from './middleware/auth';

const logger = pino({
  name: 'api-gateway',
  transport:
    process.env['NODE_ENV'] !== 'production'
      ? { target: 'pino-pretty', options: { colorize: true } }
      : undefined,
});

async function main(): Promise<void> {
  const config = loadConfig();

  // ── Express app ────────────────────────────────────────────────────────────

  const app = express();

  app.use(helmet());
  app.use(
    cors({
      origin: config.CORS_ORIGINS,
      credentials: true,
    }),
  );
  app.use(express.json({ limit: '1mb' }));
  app.use(
    rateLimit({
      windowMs: 60_000,
      max: 200,
      standardHeaders: true,
      legacyHeaders: false,
      message: { error: 'Too Many Requests' },
    }),
  );

  app.use(optionalJWT);

  // ── Routes ─────────────────────────────────────────────────────────────────

  app.use('/auth', authRouter);
  app.use('/orders', ordersRouter);
  app.use('/products', productsRouter);
  app.use('/search', searchRouter);

  // ── GraphQL ────────────────────────────────────────────────────────────────

  await setupGraphQL(app);

  // ── Health check ───────────────────────────────────────────────────────────

  app.get('/health', (_req, res) => {
    res.json({ status: 'ok' });
  });

  // ── Global error handler ───────────────────────────────────────────────────

  app.use(
    (
      err: Error,
      _req: express.Request,
      res: express.Response,
      _next: express.NextFunction,
    ) => {
      logger.error({ err }, 'Unhandled error');
      res.status(500).json({ error: 'Internal Server Error' });
    },
  );

  // ── HTTP + WebSocket server ────────────────────────────────────────────────

  const server = http.createServer(app);
  setupWebSocket(server);

  // ── Redis ──────────────────────────────────────────────────────────────────

  await redisClient.connect();
  await subscriber.connect();
  logger.info('Redis connected');

  // ── Kafka consumer (notification.events → Redis pub/sub) ──────────────────

  const kafka = new Kafka({
    clientId: 'api-gateway',
    brokers: config.KAFKA_BROKERS,
  });

  const consumer = kafka.consumer({ groupId: 'api-gateway-ws-relay' });
  await consumer.connect();
  await consumer.subscribe({ topic: 'notification.events', fromBeginning: false });

  await consumer.run({
    eachMessage: async ({ message }) => {
      if (!message.value) return;

      try {
        const event = JSON.parse(message.value.toString()) as {
          userId?: string;
          user_id?: string;
          [key: string]: unknown;
        };
        const userId = event.userId ?? event.user_id;

        if (userId) {
          await publishOrderUpdate(userId, event);
        }
      } catch (err) {
        logger.warn({ err }, 'Failed to parse Kafka message');
      }
    },
  });

  logger.info({ topic: 'notification.events' }, 'Kafka consumer running');

  // ── Start listening ────────────────────────────────────────────────────────

  server.listen(config.PORT, () => {
    logger.info({ port: config.PORT }, 'API Gateway listening');
  });

  // ── Graceful shutdown ──────────────────────────────────────────────────────

  const shutdown = async (signal: string): Promise<void> => {
    logger.info({ signal }, 'Shutting down');

    await consumer.disconnect();
    await redisClient.quit();
    await subscriber.quit();

    server.close(() => {
      logger.info('HTTP server closed');
      otelSdk.shutdown().finally(() => process.exit(0));
    });

    setTimeout(() => process.exit(1), 10_000).unref();
  };

  process.on('SIGTERM', () => void shutdown('SIGTERM'));
  process.on('SIGINT', () => void shutdown('SIGINT'));
}

main().catch((err) => {
  logger.error({ err }, 'Fatal startup error');
  process.exit(1);
});

import type { IncomingMessage } from 'http';
import * as http from 'http';
import { URL } from 'url';
import { WebSocketServer, WebSocket } from 'ws';
import jwt from 'jsonwebtoken';
import Redis from 'ioredis';
import pino from 'pino';
import { loadConfig } from '../config';
import { isTokenBlacklisted } from '../redis/client';

const logger = pino({ name: 'ws-handler' });
const config = loadConfig();

/**
 * Attaches a WebSocket server to the given HTTP server.
 * Clients must supply a valid Bearer JWT via the ?token= query parameter.
 * Each authenticated connection subscribes to "order_updates:{userId}" on Redis pub/sub.
 */
export function setupWebSocket(server: http.Server): void {
  const wss = new WebSocketServer({ server, path: '/ws/orders' });

  wss.on('connection', async (ws: WebSocket, req: IncomingMessage) => {
    let userId: string | null = null;
    let redisSubscriber: Redis | null = null;

    try {
      const url = new URL(req.url ?? '', `http://${req.headers.host}`);
      const token = url.searchParams.get('token');

      if (!token) {
        ws.close(4001, 'Missing token');
        return;
      }

      const blacklisted = await isTokenBlacklisted(token);
      if (blacklisted) {
        ws.close(4001, 'Token revoked');
        return;
      }

      let payload: { userId: string };
      try {
        payload = jwt.verify(token, config.JWT_SECRET) as { userId: string };
      } catch {
        ws.close(4001, 'Invalid token');
        return;
      }

      userId = payload.userId;
      const channel = `order_updates:${userId}`;

      redisSubscriber = new Redis(config.REDIS_URL, {
        lazyConnect: true,
        enableReadyCheck: true,
        maxRetriesPerRequest: 3,
      });

      await redisSubscriber.connect();
      await redisSubscriber.subscribe(channel);

      logger.info({ userId, channel }, 'WebSocket client subscribed');

      redisSubscriber.on('message', (_chan: string, message: string) => {
        if (ws.readyState === WebSocket.OPEN) {
          try {
            const parsed: unknown = JSON.parse(message);
            ws.send(JSON.stringify(parsed));
          } catch {
            ws.send(message);
          }
        }
      });

      ws.on('close', () => {
        logger.info({ userId }, 'WebSocket client disconnected');
        if (redisSubscriber) {
          void redisSubscriber.unsubscribe(channel).finally(() => {
            void redisSubscriber?.disconnect();
          });
        }
      });

      ws.on('error', (err) => {
        logger.error({ err, userId }, 'WebSocket error');
      });
    } catch (err) {
      logger.error({ err }, 'Unexpected error during WebSocket connection setup');
      ws.close(1011, 'Internal server error');
      if (redisSubscriber) {
        void redisSubscriber.disconnect();
      }
    }
  });
}

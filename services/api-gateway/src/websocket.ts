import type { IncomingMessage, Server } from 'http';
import { WebSocketServer, WebSocket } from 'ws';
import jwt from 'jsonwebtoken';
import Redis from 'ioredis';
import { config } from './config';
import { isTokenDenied, orderChannel } from './redis';
import type { AuthUser } from './auth';

/**
 * Real-time order updates, the last hop of:
 *
 *   Kafka (notification.events) -> gateway consumer -> Redis pub/sub -> here
 *
 * Redis pub/sub sits in the middle so it doesn't matter which gateway replica
 * holds the customer's socket: whichever one consumed the Kafka message
 * publishes to the channel, and whichever one has the socket is subscribed.
 *
 * Connect with: ws://localhost:3000/ws/orders?token=<jwt>
 */
export function setupWebSocket(server: Server): void {
  const wss = new WebSocketServer({ server, path: '/ws/orders' });

  wss.on('connection', async (socket: WebSocket, req: IncomingMessage) => {
    const user = await authenticate(req);
    if (!user) {
      socket.close(4001, 'Invalid token');
      return;
    }

    // Browsers can't set headers on a WebSocket handshake, so the token comes
    // in the query string.
    const channel = orderChannel(user.userId);

    // Each socket needs its own connection, because subscribing takes the whole
    // connection over.
    const subscriber = new Redis(config.redisUrl);
    await subscriber.subscribe(channel);

    subscriber.on('message', (_channel, payload) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(payload);
      }
    });

    socket.on('close', () => {
      void subscriber.quit();
    });
  });
}

async function authenticate(req: IncomingMessage): Promise<AuthUser | null> {
  const token = new URL(req.url ?? '', `http://${req.headers.host}`).searchParams.get('token');
  if (!token) {
    return null;
  }

  try {
    if (await isTokenDenied(token)) {
      return null;
    }
    return jwt.verify(token, config.jwtSecret) as AuthUser;
  } catch {
    return null;
  }
}

import fs from 'fs';
import path from 'path';
import type { Express, Request } from 'express';
import { ApolloServer } from '@apollo/server';
import { expressMiddleware } from '@as-integrations/express';
import { json } from 'body-parser';
import { GraphQLError } from 'graphql';
import type { AuthUser } from './auth';
import { getUser, createOrder, cancelOrder, getOrder, checkStock, type OrderItem } from './grpc';

interface Context {
  user?: AuthUser;
}

/** Throws the GraphQL equivalent of a 401 if the request is anonymous. */
function requireUser(context: Context): AuthUser {
  if (!context.user) {
    throw new GraphQLError('Not authenticated', { extensions: { code: 'UNAUTHENTICATED' } });
  }
  return context.user;
}

const resolvers = {
  Query: {
    me: (_parent: unknown, _args: unknown, context: Context) =>
      getUser({ userId: requireUser(context).userId }),

    order: (_parent: unknown, args: { orderId: string }, context: Context) => {
      requireUser(context);
      return getOrder({ orderId: args.orderId });
    },

    checkStock: (_parent: unknown, args: { productId: string; quantity: number }) =>
      checkStock(args),
  },

  Mutation: {
    createOrder: async (_parent: unknown, args: { items: OrderItem[] }, context: Context) => {
      const user = requireUser(context);
      const { orderId } = await createOrder({
        userId: user.userId,
        email: user.email,
        items: args.items,
      });

      // createOrder only returns an id and status, so read the order back to
      // return the full Order type the schema promises.
      return getOrder({ orderId });
    },

    cancelOrder: async (_parent: unknown, args: { orderId: string }, context: Context) => {
      const user = requireUser(context);
      const { success } = await cancelOrder({ orderId: args.orderId, userId: user.userId });
      return success;
    },
  },
};

/** Mounts Apollo Server at /graphql. */
export async function setupGraphQL(app: Express): Promise<void> {
  const typeDefs = fs.readFileSync(path.resolve(__dirname, '../graphql/schema.graphql'), 'utf-8');

  const server = new ApolloServer<Context>({ typeDefs, resolvers });
  await server.start();

  app.use(
    '/graphql',
    json(),
    expressMiddleware(server, {
      // optionalAuth already ran, so req.user is set for signed-in callers.
      context: async ({ req }: { req: Request }) => ({ user: req.user }),
    }),
  );
}

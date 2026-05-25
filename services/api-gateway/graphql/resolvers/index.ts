import path from 'path';
import fs from 'fs';
import type { Express, Request } from 'express';
import { ApolloServer } from '@apollo/server';
import { expressMiddleware } from '@as-integrations/express';
import { json } from 'body-parser';
import type { JwtPayload } from '../../src/middleware/auth';
import {
  registerUser,
  loginUser,
  getUser,
  createOrder,
  cancelOrder,
  getOrder,
  checkStock,
} from '../../src/grpc/clients';
import { buildUserResolvers } from './user';
import { buildOrderResolvers } from './order';

export interface GrpcClients {
  registerUser: typeof registerUser;
  loginUser: typeof loginUser;
  getUser: typeof getUser;
  createOrder: typeof createOrder;
  cancelOrder: typeof cancelOrder;
  getOrder: typeof getOrder;
  checkStock: typeof checkStock;
}

export interface GraphQLContext {
  user: JwtPayload | undefined;
  req: Request;
}

const schemaPath = path.resolve(__dirname, '../schema.graphql');
const typeDefs = fs.readFileSync(schemaPath, 'utf-8');

function mergeResolvers(
  ...resolverMaps: Array<Record<string, Record<string, unknown>>>
): Record<string, Record<string, unknown>> {
  const merged: Record<string, Record<string, unknown>> = {};

  for (const resolverMap of resolverMaps) {
    for (const [typeName, fields] of Object.entries(resolverMap)) {
      merged[typeName] = { ...(merged[typeName] ?? {}), ...fields };
    }
  }

  return merged;
}

/**
 * Creates an Apollo Server 4 instance with all resolvers and mounts it at /graphql.
 * The GraphQL context exposes the authenticated user (if any) from req.user.
 */
export async function setupGraphQL(app: Express): Promise<void> {
  const grpcClients: GrpcClients = {
    registerUser,
    loginUser,
    getUser,
    createOrder,
    cancelOrder,
    getOrder,
    checkStock,
  };

  const userResolvers = buildUserResolvers(grpcClients) as Record<
    string,
    Record<string, unknown>
  >;
  const orderResolvers = buildOrderResolvers(grpcClients) as Record<
    string,
    Record<string, unknown>
  >;

  const resolvers = mergeResolvers(userResolvers, orderResolvers);

  const server = new ApolloServer<GraphQLContext>({
    typeDefs,
    resolvers,
  });

  await server.start();

  app.use(
    '/graphql',
    json(),
    expressMiddleware(server, {
      context: async ({ req }: { req: Request }): Promise<GraphQLContext> => ({
        user: req.user,
        req,
      }),
    }),
  );
}

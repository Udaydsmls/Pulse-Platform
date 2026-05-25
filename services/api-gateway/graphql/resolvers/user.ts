import { GraphQLError } from 'graphql';
import type { GrpcClients, GraphQLContext } from './index';

/** Resolvers for User queries and auth mutations. */
export function buildUserResolvers(clients: GrpcClients) {
  return {
    Query: {
      async me(_parent: unknown, _args: unknown, context: GraphQLContext) {
        if (!context.user) {
          throw new GraphQLError('Not authenticated', {
            extensions: { code: 'UNAUTHENTICATED' },
          });
        }

        const response = await clients.getUser({ userId: context.user.userId });
        return {
          userId: response.userId,
          email: response.email,
          name: response.name,
          provider: response.provider || null,
        };
      },
    },

    Mutation: {
      async register(
        _parent: unknown,
        args: { email: string; password: string; name: string },
      ) {
        const response = await clients.registerUser(args);
        return { token: response.token, userId: response.userId };
      },

      async login(
        _parent: unknown,
        args: { email: string; password: string },
      ) {
        const response = await clients.loginUser(args);
        return { token: response.token, userId: response.userId };
      },
    },
  };
}

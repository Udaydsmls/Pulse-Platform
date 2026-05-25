import { GraphQLError } from 'graphql';
import type { GrpcClients, GraphQLContext } from './index';

interface OrderItemInput {
  productId: string;
  quantity: number;
  unitPrice: number;
}

/** Resolvers for Order queries and mutations. */
export function buildOrderResolvers(clients: GrpcClients) {
  return {
    Query: {
      async order(
        _parent: unknown,
        args: { orderId: string },
        context: GraphQLContext,
      ) {
        if (!context.user) {
          throw new GraphQLError('Not authenticated', {
            extensions: { code: 'UNAUTHENTICATED' },
          });
        }

        return clients.getOrder({ orderId: args.orderId });
      },

      async checkStock(
        _parent: unknown,
        args: { productId: string; quantity: number },
      ) {
        return clients.checkStock({ productId: args.productId, quantity: args.quantity });
      },
    },

    Mutation: {
      async createOrder(
        _parent: unknown,
        args: { items: OrderItemInput[] },
        context: GraphQLContext,
      ) {
        if (!context.user) {
          throw new GraphQLError('Not authenticated', {
            extensions: { code: 'UNAUTHENTICATED' },
          });
        }

        const { userId } = context.user;
        const response = await clients.createOrder({
          userId,
          tenantId: userId,
          items: args.items.map((item) => ({
            productId: item.productId,
            quantity: item.quantity,
            unitPrice: item.unitPrice,
          })),
        });

        return clients.getOrder({ orderId: response.orderId });
      },

      async cancelOrder(
        _parent: unknown,
        args: { orderId: string },
        context: GraphQLContext,
      ) {
        if (!context.user) {
          throw new GraphQLError('Not authenticated', {
            extensions: { code: 'UNAUTHENTICATED' },
          });
        }

        const response = await clients.cancelOrder({
          orderId: args.orderId,
          userId: context.user.userId,
        });

        return response.success;
      },
    },
  };
}

import path from 'path';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import { config } from './config';

/**
 * gRPC clients for the Go services.
 *
 * The .proto files are loaded at runtime rather than compiled to TypeScript, so
 * there is no codegen step here — but it also means the request and response
 * types below are hand-written and must match proto/.
 */

// Repo root proto/ when running from src/ or dist/. The Docker image copies the
// protos in and sets PROTO_DIR, since the repo layout isn't there.
const PROTO_DIR = process.env.PROTO_DIR ?? path.resolve(__dirname, '../../../proto');

function connect(file: string, pkg: string, service: string, address: string): grpc.Client {
  // keepCase: false converts snake_case proto fields to camelCase in JS.
  const definition = protoLoader.loadSync(path.join(PROTO_DIR, file), {
    keepCase: false,
    defaults: true,
  });

  const loaded = grpc.loadPackageDefinition(definition) as unknown as Record<
    string,
    Record<string, Record<string, grpc.ServiceClientConstructor>>
  >;
  const Client = loaded[pkg].v1[service];

  // Istio terminates mTLS between pods, so the application connects in plaintext.
  return new Client(address, grpc.credentials.createInsecure());
}

const userClient = connect('user.proto', 'user', 'UserService', config.userServiceAddr);
const orderClient = connect('order.proto', 'order', 'OrderService', config.orderServiceAddr);
const inventoryClient = connect(
  'inventory.proto',
  'inventory',
  'InventoryService',
  config.inventoryServiceAddr,
);

/**
 * The shape proto-loader gives every unary method it puts on the client.
 * Methods are looked up by name, so they aren't statically typed.
 */
type UnaryMethod = (
  request: unknown,
  callback: (err: grpc.ServiceError | null, response: unknown) => void,
) => void;

/** Wraps a callback-style gRPC method in a promise. */
function call<TResponse>(client: grpc.Client, method: string, request: unknown): Promise<TResponse> {
  return new Promise((resolve, reject) => {
    const fn = (client as unknown as Record<string, UnaryMethod>)[method];
    fn.call(client, request, (err, response) => {
      if (err) reject(err);
      else resolve(response as TResponse);
    });
  });
}

// ── Types ────────────────────────────────────────────────────────────────────

export interface OrderItem {
  productId: string;
  quantity: number;
  unitPrice: number;
}

export interface AuthResult {
  token: string;
  userId: string;
}

export interface UserProfile {
  userId: string;
  email: string;
  name: string;
  createdAt: string;
}

export interface Order {
  orderId: string;
  userId: string;
  status: string;
  items: OrderItem[];
  total: number;
  createdAt: string;
}

export interface StockStatus {
  available: boolean;
  stockLevel: number;
}

// ── Calls ────────────────────────────────────────────────────────────────────

export const register = (req: { email: string; password: string; name: string }) =>
  call<AuthResult>(userClient, 'register', req);

export const login = (req: { email: string; password: string }) =>
  call<AuthResult>(userClient, 'login', req);

export const getUser = (req: { userId: string }) =>
  call<UserProfile>(userClient, 'getUser', req);

/** Starts the saga. email is carried along so notifications need no extra lookup. */
export const createOrder = (req: { userId: string; email: string; items: OrderItem[] }) =>
  call<{ orderId: string; status: string }>(orderClient, 'createOrder', req);

export const cancelOrder = (req: { orderId: string; userId: string }) =>
  call<{ success: boolean }>(orderClient, 'cancelOrder', req);

export const getOrder = (req: { orderId: string }) =>
  call<Order>(orderClient, 'getOrder', req);

export const checkStock = (req: { productId: string; quantity: number }) =>
  call<StockStatus>(inventoryClient, 'checkStock', req);

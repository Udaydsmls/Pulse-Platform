import path from 'path';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import { loadConfig } from '../config';

const PROTO_DIR = path.resolve(__dirname, '../../../proto');

const LOADER_OPTIONS: protoLoader.Options = {
  keepCase: false,
  longs: 'String',
  enums: 'String',
  defaults: true,
  oneofs: true,
};

function loadPackage(protoFile: string): protoLoader.PackageDefinition {
  return protoLoader.loadSync(path.join(PROTO_DIR, protoFile), LOADER_OPTIONS);
}

const userPkg = grpc.loadPackageDefinition(loadPackage('user.proto')) as unknown as UserProto;
const orderPkg = grpc.loadPackageDefinition(loadPackage('order.proto')) as unknown as OrderProto;
const inventoryPkg = grpc.loadPackageDefinition(
  loadPackage('inventory.proto'),
) as unknown as InventoryProto;

// ──────────────────────────────────────────────────────────────────────────────
// Proto type shims (dynamic loading — no codegen)
// ──────────────────────────────────────────────────────────────────────────────

interface UserProto {
  'user': {
    v1: {
      UserService: grpc.ServiceClientConstructor;
    };
  };
}

interface OrderProto {
  'order': {
    v1: {
      OrderService: grpc.ServiceClientConstructor;
    };
  };
}

interface InventoryProto {
  'inventory': {
    v1: {
      InventoryService: grpc.ServiceClientConstructor;
    };
  };
}

// ──────────────────────────────────────────────────────────────────────────────
// Message types
// ──────────────────────────────────────────────────────────────────────────────

export interface RegisterRequest { email: string; password: string; name: string }
export interface RegisterResponse { userId: string; token: string }

export interface LoginRequest { email: string; password: string }
export interface LoginResponse { token: string; userId: string }

export interface GetUserRequest { userId: string }
export interface GetUserResponse {
  userId: string;
  email: string;
  name: string;
  createdAt: string;
  provider: string;
}

export interface OAuthLoginRequest { code: string; provider: string }
export interface OAuthLoginResponse { token: string; userId: string }

export interface OrderItem { productId: string; quantity: number; unitPrice: number }

export interface CreateOrderRequest { userId: string; tenantId: string; items: OrderItem[] }
export interface CreateOrderResponse { orderId: string; status: string }

export interface CancelOrderRequest { orderId: string; userId: string }
export interface CancelOrderResponse { success: boolean }

export interface GetOrderRequest { orderId: string }
export interface GetOrderResponse {
  orderId: string;
  userId: string;
  status: string;
  items: OrderItem[];
  total: number;
  createdAt: string;
}

export interface CheckStockRequest { productId: string; quantity: number }
export interface CheckStockResponse { available: boolean; stockLevel: number }

export interface ReserveStockRequest { orderId: string; productId: string; quantity: number }
export interface ReserveStockResponse { success: boolean; reservationId: string }

// ──────────────────────────────────────────────────────────────────────────────
// gRPC client instances
// ──────────────────────────────────────────────────────────────────────────────

const config = loadConfig();
const insecure = grpc.credentials.createInsecure();

export const UserServiceClient = new userPkg['user'].v1.UserService(
  config.GRPC_USER_SERVICE_ADDR,
  insecure,
);

export const OrderServiceClient = new orderPkg['order'].v1.OrderService(
  config.GRPC_ORDER_SERVICE_ADDR,
  insecure,
);

export const InventoryServiceClient = new inventoryPkg['inventory'].v1.InventoryService(
  config.GRPC_INVENTORY_SERVICE_ADDR,
  insecure,
);

// ──────────────────────────────────────────────────────────────────────────────
// Promisify helper
// ──────────────────────────────────────────────────────────────────────────────

/**
 * Wraps a gRPC unary call in a Promise.
 */
function promisifyGrpcCall<TReq, TRes>(
  client: grpc.Client,
  methodName: string,
  request: TReq,
): Promise<TRes> {
  return new Promise<TRes>((resolve, reject) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (client as any)[methodName](
      request,
      (err: grpc.ServiceError | null, response: TRes) => {
        if (err) {
          reject(err);
        } else {
          resolve(response);
        }
      },
    );
  });
}

// ──────────────────────────────────────────────────────────────────────────────
// Typed wrapper functions
// ──────────────────────────────────────────────────────────────────────────────

/** Registers a new user and returns userId + JWT token. */
export function registerUser(req: RegisterRequest): Promise<RegisterResponse> {
  return promisifyGrpcCall<RegisterRequest, RegisterResponse>(UserServiceClient, 'register', req);
}

/** Authenticates a user and returns a JWT token. */
export function loginUser(req: LoginRequest): Promise<LoginResponse> {
  return promisifyGrpcCall<LoginRequest, LoginResponse>(UserServiceClient, 'login', req);
}

/** Fetches a user profile by userId. */
export function getUser(req: GetUserRequest): Promise<GetUserResponse> {
  return promisifyGrpcCall<GetUserRequest, GetUserResponse>(UserServiceClient, 'getUser', req);
}

/** Handles OAuth login flow for Google and GitHub providers. */
export function oauthLogin(req: OAuthLoginRequest): Promise<OAuthLoginResponse> {
  return promisifyGrpcCall<OAuthLoginRequest, OAuthLoginResponse>(
    UserServiceClient,
    'oAuthLogin',
    req,
  );
}

/** Creates a new order on behalf of a user. */
export function createOrder(req: CreateOrderRequest): Promise<CreateOrderResponse> {
  return promisifyGrpcCall<CreateOrderRequest, CreateOrderResponse>(
    OrderServiceClient,
    'createOrder',
    req,
  );
}

/** Cancels an existing order. */
export function cancelOrder(req: CancelOrderRequest): Promise<CancelOrderResponse> {
  return promisifyGrpcCall<CancelOrderRequest, CancelOrderResponse>(
    OrderServiceClient,
    'cancelOrder',
    req,
  );
}

/** Retrieves order details by orderId. */
export function getOrder(req: GetOrderRequest): Promise<GetOrderResponse> {
  return promisifyGrpcCall<GetOrderRequest, GetOrderResponse>(
    OrderServiceClient,
    'getOrder',
    req,
  );
}

/** Checks stock availability for a product. */
export function checkStock(req: CheckStockRequest): Promise<CheckStockResponse> {
  return promisifyGrpcCall<CheckStockRequest, CheckStockResponse>(
    InventoryServiceClient,
    'checkStock',
    req,
  );
}

/** Reserves stock for an order. */
export function reserveStock(req: ReserveStockRequest): Promise<ReserveStockResponse> {
  return promisifyGrpcCall<ReserveStockRequest, ReserveStockResponse>(
    InventoryServiceClient,
    'reserveStock',
    req,
  );
}

import { config } from './config';

// Calls to the Go services. They all speak JSON over HTTP, so one helper
// covers every call.

export class ServiceError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  });

  const body = await response.json().catch(() => ({}));

  if (!response.ok) {
    // Pass the service's own status and message through to the client.
    throw new ServiceError(response.status, (body as { error?: string }).error ?? 'Request failed');
  }
  return body as T;
}

function post<T>(url: string, payload: unknown): Promise<T> {
  return request<T>(url, { method: 'POST', body: JSON.stringify(payload) });
}

export interface OrderItem {
  productId: string;
  quantity: number;
  unitPrice: number;
}

export interface AuthResult {
  userId: string;
  token: string;
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

export const register = (body: { email: string; password: string; name: string }) =>
  post<AuthResult>(`${config.userServiceUrl}/register`, body);

export const login = (body: { email: string; password: string }) =>
  post<AuthResult>(`${config.userServiceUrl}/login`, body);

export const createOrder = (body: { userId: string; email: string; items: OrderItem[] }) =>
  post<{ orderId: string; status: string }>(`${config.orderServiceUrl}/orders`, body);

export const cancelOrder = (orderId: string, userId: string) =>
  post<{ success: boolean }>(`${config.orderServiceUrl}/orders/${orderId}/cancel`, { userId });

export const getOrder = (orderId: string) =>
  request<Order>(`${config.orderServiceUrl}/orders/${orderId}`);

export const checkStock = (productId: string, quantity: number) =>
  request<StockStatus>(`${config.inventoryServiceUrl}/stock/${productId}?quantity=${quantity}`);

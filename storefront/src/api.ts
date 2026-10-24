import { Product, Cart, Order, Review, LatencyMetric } from './types';

const API_BASE = '/api';

let lastMetricListener: ((metric: LatencyMetric) => void) | null = null;

export function setLatencyListener(listener: (metric: LatencyMetric) => void) {
  lastMetricListener = listener;
}

async function fetchWithMetrics<T>(url: string, options?: RequestInit): Promise<T> {
  const start = performance.now();
  let status = 0;
  let errorMsg: string | undefined;

  try {
    const res = await fetch(url, options);
    status = res.status;
    const duration = Math.round(performance.now() - start);

    if (!res.ok) {
      let bodyText = '';
      try {
        const errJson = await res.json();
        bodyText = errJson.error || JSON.stringify(errJson);
      } catch {
        bodyText = await res.text();
      }
      errorMsg = bodyText || `HTTP ${res.status}`;
      if (lastMetricListener) {
        lastMetricListener({
          endpoint: url.replace(API_BASE, ''),
          durationMs: duration,
          status,
          timestamp: new Date().toLocaleTimeString(),
          error: errorMsg,
        });
      }
      throw new Error(errorMsg);
    }

    const data = await res.json();
    if (lastMetricListener) {
      lastMetricListener({
        endpoint: url.replace(API_BASE, ''),
        durationMs: duration,
        status,
        timestamp: new Date().toLocaleTimeString(),
      });
    }
    return data as T;
  } catch (err: any) {
    if (!errorMsg) {
      const duration = Math.round(performance.now() - start);
      errorMsg = err.message || 'Network error';
      if (lastMetricListener) {
        lastMetricListener({
          endpoint: url.replace(API_BASE, ''),
          durationMs: duration,
          status: status || 500,
          timestamp: new Date().toLocaleTimeString(),
          error: errorMsg,
        });
      }
    }
    throw err;
  }
}

export const api = {
  getProducts: () => fetchWithMetrics<Product[]>(`${API_BASE}/catalog/products`),
  getProduct: (id: string) => fetchWithMetrics<Product>(`${API_BASE}/catalog/products/${id}`),
  searchProducts: (q: string, category?: string) => {
    const params = new URLSearchParams();
    if (q) params.set('q', q);
    if (category) params.set('category', category);
    return fetchWithMetrics<{ results: Product[]; count: number }>(`${API_BASE}/search/search?${params.toString()}`);
  },
  getCart: (cartId: string) => fetchWithMetrics<Cart>(`${API_BASE}/cart/cart/${cartId}`),
  addToCart: (cartId: string, item: { product_id: string; name: string; price: number; quantity: number }) =>
    fetchWithMetrics<Cart>(`${API_BASE}/cart/cart/${cartId}/items`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(item),
    }),
  removeFromCart: (cartId: string, productId: string) =>
    fetchWithMetrics<Cart>(`${API_BASE}/cart/cart/${cartId}/items/${productId}`, {
      method: 'DELETE',
    }),
  clearCart: (cartId: string) =>
    fetchWithMetrics<{ status: string }>(`${API_BASE}/cart/cart/${cartId}`, {
      method: 'DELETE',
    }),
  checkout: (payload: {
    cart_id: string;
    customer_name: string;
    customer_email: string;
    shipping_address: string;
    card_token: string;
  }) =>
    fetchWithMetrics<Order>(`${API_BASE}/checkout/checkout`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    }),
  getOrder: (orderId: string) => fetchWithMetrics<Order>(`${API_BASE}/orders/orders/${orderId}`),
  getReviews: (productId: string) => fetchWithMetrics<Review[]>(`${API_BASE}/reviews/reviews/product/${productId}`),
  submitReview: (review: { product_id: string; rating: number; comment: string; author: string }) =>
    fetchWithMetrics<Review>(`${API_BASE}/reviews/reviews`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(review),
    }),
};

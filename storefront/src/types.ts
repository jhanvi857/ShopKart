export interface Product {
  id: string;
  name: string;
  description: string;
  brand: string;
  mrp: number;
  price: number;
  category: string;
  rating: number;
  rating_count: number;
  specs: string;
  image_url: string;
  created_at: string;
}

export interface InventoryStatus {
  product_id: string;
  stock: number;
  reserved: number;
  available: number;
  updated_at: string;
}

export interface CartItem {
  product_id: string;
  name: string;
  price: number;
  quantity: number;
  image_url?: string;
  mrp?: number;
}

export interface Cart {
  cart_id: string;
  items: CartItem[];
  total: number;
  updated_at: string;
}

export interface Review {
  id: string;
  product_id: string;
  rating: number;
  comment: string;
  author: string;
  created_at: string;
}

export interface OrderItem {
  product_id: string;
  name: string;
  price: number;
  quantity: number;
}

export interface Order {
  order_id: string;
  customer_name: string;
  customer_email: string;
  shipping_address: string;
  total_amount: number;
  currency: string;
  payment_id: string;
  transaction_id: string;
  status: string;
  items: OrderItem[];
  created_at: string;
}

export interface LatencyMetric {
  endpoint: string;
  durationMs: number;
  status: number;
  timestamp: string;
  error?: string;
}

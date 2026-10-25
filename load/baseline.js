import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 8 },  // ramp to steady low load
    { duration: '2m', target: 10 },  // steady background shopping load
    { duration: '30s', target: 0 },   // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<600'], // 95% of requests under 600ms under steady load
    http_req_failed: ['rate<0.02'],    // error rate below 2%
  },
};

const BASE_URL = __ENV.GATEWAY_URL || 'http://localhost:9000';

const CATALOG_ITEMS = [
  { id: 'el_01', name: 'Sony WH-1000XM5 Wireless Noise Cancelling Headphones', price: 29990, brand: 'Sony', cat: 'Electronics' },
  { id: 'el_02', name: 'boAt Rockerz 450 Bluetooth On-Ear Headphones', price: 1499, brand: 'boAt', cat: 'Electronics' },
  { id: 'mb_01', name: 'Apple iPhone 15 (128 GB, Blue)', price: 71999, brand: 'Apple', cat: 'Mobiles & Accessories' },
  { id: 'mb_04', name: 'OnePlus Nord CE 3 Lite 5G (Pastel Lime, 128 GB)', price: 19999, brand: 'OnePlus', cat: 'Mobiles & Accessories' },
  { id: 'fa_01', name: "Levi's Men 511 Slim Fit Stretchable Jeans", price: 2199, brand: "Levi's", cat: 'Fashion & Apparel' },
  { id: 'hk_01', name: 'Prestige Iris 750 Watt Mixer Grinder with 3 Jars', price: 3199, brand: 'Prestige', cat: 'Home & Kitchen' },
  { id: 'gr_01', name: 'Tata Tea Gold 1 kg Rich Taste and Aroma', price: 480, brand: 'Tata', cat: 'Grocery & Essentials' },
  { id: 'sp_01', name: 'Boldfit Bold Rubber Hex Dumbbell 5kg Pair', price: 1899, brand: 'Boldfit', cat: 'Sports & Fitness' },
];

const SEARCH_QUERIES = ['sony', 'boat', 'wireless', 'mixer', 'tea', 'iphone', 'jeans', 'dumbbell'];

const INDIAN_NAMES = ['Aarav Sharma', 'Priya Patel', 'Rohan Gupta', 'Neha Verma', 'Vikram Malhotra', 'Ananya Iyer'];
const INDIAN_ADDRESSES = [
  'Flat 402, Green Glen Layout, Bellandur, Bengaluru 560103',
  '12/B Pocket A, Mayur Vihar Phase 2, New Delhi 110091',
  'Row House 7, Amanora Park Town, Hadapsar, Pune 411028',
  'Flat 101, Palm Beach Residency, Sector 14, Navi Mumbai 400705',
];

export default function () {
  const item = CATALOG_ITEMS[Math.floor(Math.random() * CATALOG_ITEMS.length)];
  const query = SEARCH_QUERIES[Math.floor(Math.random() * SEARCH_QUERIES.length)];
  const customerName = INDIAN_NAMES[Math.floor(Math.random() * INDIAN_NAMES.length)];
  const address = INDIAN_ADDRESSES[Math.floor(Math.random() * INDIAN_ADDRESSES.length)];
  const cartId = `baseline_cart_vu${__VU}_it${__ITER}_${Date.now().toString(36)}`;

  // 1. Browse / Search
  const searchRes = http.get(`${BASE_URL}/api/search/search?q=${query}`, {
    tags: { name: '01_search_browse' },
  });
  check(searchRes, {
    'search status 200': (r) => r.status === 200,
  });

  // Realistic human think-time while reading search results (1.5 - 2.5s)
  sleep(1.5 + Math.random());

  // 2. Product Detail & Verified Reviews
  const detailRes = http.get(`${BASE_URL}/api/catalog/catalog/products/${item.id}`, {
    tags: { name: '02_product_detail' },
  });
  check(detailRes, {
    'product detail status 200': (r) => r.status === 200,
  });

  const reviewsRes = http.get(`${BASE_URL}/api/reviews/reviews/product/${item.id}`, {
    tags: { name: '02_fetch_reviews' },
  });
  check(reviewsRes, {
    'reviews status 200': (r) => r.status === 200,
  });

  // Think-time reading specifications and customer reviews (1.5 - 2.5s)
  sleep(1.5 + Math.random());

  // 3. Add to Cart
  const addPayload = JSON.stringify({
    product_id: item.id,
    name: item.name,
    price: item.price,
    quantity: 1,
  });

  const addRes = http.post(`${BASE_URL}/api/cart/cart/${cartId}/items`, addPayload, {
    headers: { 'Content-Type': 'application/json' },
    tags: { name: '03_add_to_cart' },
  });
  check(addRes, {
    'add to cart status 200': (r) => r.status === 200,
  });

  // Think-time navigating to checkout (1.0 - 2.0s)
  sleep(1.0 + Math.random());

  // 4. Place Order (Checkout -> Inventory lock -> Payments charge -> Postgres DB -> Kafka)
  const checkoutPayload = JSON.stringify({
    cart_id: cartId,
    customer_name: customerName,
    customer_email: `${customerName.toLowerCase().replace(/\s+/g, '.')}@example.in`,
    shipping_address: address,
    card_token: 'tok_visa_valid',
  });

  const checkoutRes = http.post(`${BASE_URL}/api/checkout/checkout`, checkoutPayload, {
    headers: { 'Content-Type': 'application/json' },
    tags: { name: '04_checkout' },
  });

  const checkoutOk = check(checkoutRes, {
    'checkout status 201': (r) => r.status === 201,
  });

  // 5. Order Status Polling (if checkout succeeded)
  if (checkoutOk && checkoutRes.status === 201) {
    try {
      const order = JSON.parse(checkoutRes.body);
      if (order && order.order_id) {
        sleep(0.5); // Short wait before checking order status
        const pollRes = http.get(`${BASE_URL}/api/checkout/orders/${order.order_id}`, {
          tags: { name: '05_poll_order' },
        });
        check(pollRes, {
          'order status polled 200': (r) => r.status === 200,
        });
      }
    } catch (e) {
      // Body parse error
    }
  }

  // Inter-journey think-time (2.0 - 4.0s)
  sleep(2.0 + Math.random() * 2);
}

export function handleSummary(data) {
  const p95 = data.metrics.http_req_duration ? data.metrics.http_req_duration.values['p(95)'].toFixed(2) : 'N/A';
  const failRate = data.metrics.http_req_failed ? (data.metrics.http_req_failed.values.rate * 100).toFixed(2) : '0.00';
  const totalReqs = data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0;

  console.log('\n========================================');
  console.log('SHOPKART BASELINE LOAD JOURNEY SUMMARY');
  console.log('========================================');
  console.log(`Total Requests Completed: ${totalReqs}`);
  console.log(`P95 Latency:             ${p95} ms (Threshold: < 600 ms)`);
  console.log(`HTTP Error Rate:         ${failRate}% (Threshold: < 2.0%)`);
  console.log('========================================\n');

  return {
    stdout: JSON.stringify(data),
  };
}

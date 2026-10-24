import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 10 },
    { duration: '30s', target: 40 }, // flash sale rush on catalog cache & checkout
    { duration: '15s', target: 10 },
    { duration: '5s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    http_req_failed: ['rate<0.10'],
  },
};

const BASE_URL = __ENV.GATEWAY_URL || 'http://localhost:9000';
const ADMIN_KEY = __ENV.ADMIN_API_KEY || 'shopkart-ops-secret-key-prod-998';

const FLASH_ITEMS = [
  { id: 'el_01', name: 'Sony WH-1000XM5 Wireless Headphones', price: 29990 },
  { id: 'mb_04', name: 'OnePlus Nord CE 3 Lite 5G', price: 19999 },
];

export function setup() {
  // Evict catalog cache at start of flash sale event to test cache stampede behavior
  http.post(`${BASE_URL}/api/catalog/admin/cache/flush`, '{}', {
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Key': ADMIN_KEY,
    },
  });
}

export default function () {
  const item = FLASH_ITEMS[__ITER % FLASH_ITEMS.length];
  const cartId = `flash_cart_vu${__VU}_it${__ITER}_${Date.now().toString(36)}`;

  // 1. Browse / Search flash sale item
  const browseRes = http.get(`${BASE_URL}/api/search/search?q=${item.id === 'el_01' ? 'sony' : 'oneplus'}`, {
    tags: { name: '01_flash_search' },
  });
  check(browseRes, {
    'flash search 200': (r) => r.status === 200,
  });

  // Short flash sale think time
  sleep(0.4 + Math.random() * 0.4);

  // 2. Product Detail & Reviews
  const detailRes = http.get(`${BASE_URL}/api/catalog/catalog/products/${item.id}`, {
    tags: { name: '02_flash_detail' },
  });
  check(detailRes, {
    'flash detail 200': (r) => r.status === 200,
  });

  const revRes = http.get(`${BASE_URL}/api/reviews/reviews/product/${item.id}`, {
    tags: { name: '02_flash_reviews' },
  });
  check(revRes, {
    'flash reviews 200': (r) => r.status === 200,
  });

  // 3. Add to Cart
  const addRes = http.post(
    `${BASE_URL}/api/cart/cart/${cartId}/items`,
    JSON.stringify({
      product_id: item.id,
      name: item.name,
      price: item.price,
      quantity: 1,
    }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: '03_flash_cart' },
    }
  );
  check(addRes, {
    'flash add cart 200': (r) => r.status === 200,
  });

  sleep(0.5 + Math.random() * 0.5);

  // 4. Flash Checkout
  const checkoutRes = http.post(
    `${BASE_URL}/api/checkout/checkout`,
    JSON.stringify({
      cart_id: cartId,
      customer_name: `Flash Buyer ${__VU}`,
      customer_email: `buyer_${__VU}@flash-sale.in`,
      shipping_address: 'Flat 12, Express Towers, Nariman Point, Mumbai 400021',
      card_token: 'tok_flash_visa',
    }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: '04_flash_checkout' },
    }
  );

  const checkoutOk = check(checkoutRes, {
    'checkout handled': (r) => r.status === 201 || r.status === 503 || r.status === 409,
  });

  // 5. Poll Order Status
  if (checkoutOk && checkoutRes.status === 201) {
    try {
      const order = JSON.parse(checkoutRes.body);
      if (order && order.order_id) {
        sleep(0.3);
        const pollRes = http.get(`${BASE_URL}/api/checkout/orders/${order.order_id}`, {
          tags: { name: '05_flash_poll' },
        });
        check(pollRes, { 'order poll 200': (r) => r.status === 200 });
      }
    } catch (e) {}
  }

  sleep(1.0 + Math.random());
}

export function handleSummary(data) {
  const p95 = data.metrics.http_req_duration ? data.metrics.http_req_duration.values['p(95)'].toFixed(2) : 'N/A';
  const failRate = data.metrics.http_req_failed ? (data.metrics.http_req_failed.values.rate * 100).toFixed(2) : '0.00';
  const totalReqs = data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0;

  console.log('\n========================================');
  console.log('SHOPKART FLASH SALE CATALOG LOAD RESULTS');
  console.log('========================================');
  console.log(`Total Requests Handled:  ${totalReqs}`);
  console.log(`P95 Latency:             ${p95} ms (Threshold: < 1000 ms)`);
  console.log(`HTTP Error Rate:         ${failRate}% (Threshold: < 10.0%)`);
  console.log('========================================\n');

  return { stdout: null };
}

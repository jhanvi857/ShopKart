import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '15s', target: 5 },   // warm-up
    { duration: '20s', target: 35 },  // aggressive ramp-up
    { duration: '30s', target: 50 },  // peak sustained spike pressure
    { duration: '15s', target: 10 },  // ramp down
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<1500'], // Under heavy spike contention, p95 must stay under 1500ms
    http_req_failed: ['rate<0.15'],    // Total error rate (including pool rejects) under 15%
  },
};

const BASE_URL = __ENV.GATEWAY_URL || 'http://localhost:9000';

const HIGH_DEMAND_ITEMS = [
  { id: 'el_01', name: 'Sony WH-1000XM5 Wireless Headphones', price: 29990 },
  { id: 'mb_04', name: 'OnePlus Nord CE 3 Lite 5G (128 GB)', price: 19999 },
  { id: 'el_02', name: 'boAt Rockerz 450 Bluetooth Headphones', price: 1499 },
  { id: 'fa_01', name: "Levi's Men 511 Slim Fit Jeans", price: 2199 },
];

const CUSTOMERS = [
  { name: 'Kavita Sundaram', address: 'Flat 6B, Prestige Palms, Whitefield, Bengaluru 560066' },
  { name: 'Devendra Joshi', address: '45 Lake View Enclave, HSR Layout, Bengaluru 560102' },
  { name: 'Sameer Bansal', address: 'B-204, Ganga Satellite, Wanowrie, Pune 411040' },
  { name: 'Ritu Sen', address: 'Tower 4 Flat 901, DLF Phase 5, Gurugram 122002' },
];

export default function () {
  const item = HIGH_DEMAND_ITEMS[Math.floor(Math.random() * HIGH_DEMAND_ITEMS.length)];
  const customer = CUSTOMERS[Math.floor(Math.random() * CUSTOMERS.length)];
  const cartId = `spike_cart_vu${__VU}_it${__ITER}_${Date.now().toString(36)}`;

  // Step 1: Browse / Search spike target
  const searchRes = http.get(`${BASE_URL}/api/search/search?q=${item.name.split(' ')[0]}`, {
    tags: { name: '01_spike_browse' },
  });
  check(searchRes, {
    'browse ok': (r) => r.status === 200,
  });

  // Short think-time during flash rush (0.3 - 0.8s)
  sleep(0.3 + Math.random() * 0.5);

  // Step 2: Product Detail & Reviews
  const detailRes = http.get(`${BASE_URL}/api/catalog/catalog/products/${item.id}`, {
    tags: { name: '02_spike_detail' },
  });
  check(detailRes, {
    'detail ok': (r) => r.status === 200,
  });

  // Step 3: Add to Cart
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
      tags: { name: '03_spike_add_cart' },
    }
  );
  check(addRes, {
    'cart add ok': (r) => r.status === 200,
  });

  // Rapid checkout jump (0.4 - 0.9s think-time)
  sleep(0.4 + Math.random() * 0.5);

  // Step 4: Checkout (Ramps load through checkout -> inventory row locks -> payments HTTP -> postgres DB -> kafka)
  const checkoutPayload = JSON.stringify({
    cart_id: cartId,
    customer_name: customer.name,
    customer_email: `${customer.name.toLowerCase().replace(/\s+/g, '.')}@spike-test.in`,
    shipping_address: customer.address,
    card_token: 'tok_spike_rush_visa',
  });

  const checkoutRes = http.post(`${BASE_URL}/api/checkout/checkout`, checkoutPayload, {
    headers: { 'Content-Type': 'application/json' },
    tags: { name: '04_spike_checkout' },
  });

  // During heavy spike contention, responses will be 201 Created or 503 Service Unavailable / 409 Conflict
  const checkoutHandled = check(checkoutRes, {
    'checkout handled by system (201, 409, 503)': (r) =>
      r.status === 201 || r.status === 409 || r.status === 503,
  });

  // Step 5: Order status poll if successfully placed
  if (checkoutHandled && checkoutRes.status === 201) {
    try {
      const order = JSON.parse(checkoutRes.body);
      if (order && order.order_id) {
        sleep(0.5);
        const pollRes = http.get(`${BASE_URL}/api/checkout/orders/${order.order_id}`, {
          tags: { name: '05_spike_poll_order' },
        });
        check(pollRes, {
          'order poll ok': (r) => r.status === 200,
        });
      }
    } catch (e) {
      // Body parse ignore
    }
  }

  // Brief pause before next shopper attempt
  sleep(1.0 + Math.random());
}

export function handleSummary(data) {
  const p95 = data.metrics.http_req_duration ? data.metrics.http_req_duration.values['p(95)'].toFixed(2) : 'N/A';
  const p99 = data.metrics.http_req_duration ? data.metrics.http_req_duration.values['p(99)'].toFixed(2) : 'N/A';
  const failRate = data.metrics.http_req_failed ? (data.metrics.http_req_failed.values.rate * 100).toFixed(2) : '0.00';
  const totalReqs = data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0;

  console.log('\n========================================');
  console.log('SHOPKART SPIKE CHECKOUT LOAD RESULTS');
  console.log('========================================');
  console.log(`Total Requests Handled:  ${totalReqs}`);
  console.log(`P95 Latency:             ${p95} ms (Threshold: < 1500 ms)`);
  console.log(`P99 Latency:             ${p99} ms`);
  console.log(`HTTP Error / Reject Rate:${failRate}% (Threshold: < 15.0%)`);
  console.log('========================================\n');

  return {
    stdout: null,
  };
}

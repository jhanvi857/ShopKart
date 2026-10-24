import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 15 },
    { duration: '35s', target: 35 }, // sustained flow of completed orders into Kafka queue
    { duration: '15s', target: 10 },
    { duration: '5s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<1200'],
    http_req_failed: ['rate<0.10'],
  },
};

const BASE_URL = __ENV.GATEWAY_URL || 'http://localhost:9000';

const ITEMS = [
  { id: 'gr_01', name: 'Tata Tea Gold 1 kg', price: 480 },
  { id: 'fa_02', name: 'Roadster Pure Cotton Casual Shirt', price: 699 },
  { id: 'sp_01', name: 'Boldfit Bold Rubber Hex Dumbbell 5kg Pair', price: 1899 },
  { id: 'hk_03', name: 'Milton Thermosteel Flip Lid Flask 1000ml', price: 999 },
];

export default function () {
  const item = ITEMS[Math.floor(Math.random() * ITEMS.length)];
  const cartId = `queue_cart_vu${__VU}_it${__ITER}_${Date.now().toString(36)}`;

  // 1. Browse / Search
  const searchRes = http.get(`${BASE_URL}/api/search/search?q=${item.name.split(' ')[0]}`, {
    tags: { name: '01_queue_search' },
  });
  check(searchRes, { 'search ok': (r) => r.status === 200 });

  sleep(0.3 + Math.random() * 0.3);

  // 2. Product Detail & Reviews
  const detailRes = http.get(`${BASE_URL}/api/catalog/catalog/products/${item.id}`, {
    tags: { name: '02_queue_detail' },
  });
  check(detailRes, { 'detail ok': (r) => r.status === 200 });

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
      tags: { name: '03_queue_cart' },
    }
  );
  check(addRes, { 'cart add ok': (r) => r.status === 200 });

  sleep(0.3 + Math.random() * 0.3);

  // 4. Checkout (Pushes order event into Kafka shopkart-orders)
  const checkoutRes = http.post(
    `${BASE_URL}/api/checkout/checkout`,
    JSON.stringify({
      cart_id: cartId,
      customer_name: `Queue Runner ${__VU}`,
      customer_email: `runner_${__VU}@shopkart-queue.in`,
      shipping_address: '404 Kafka Terminal Way, Outer Ring Road, Bengaluru 560103',
      card_token: 'tok_queue_card',
    }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: '04_queue_checkout' },
    }
  );

  const checkoutOk = check(checkoutRes, {
    'checkout handled': (r) => r.status === 201 || r.status === 503,
  });

  // 5. Poll Order Status
  if (checkoutOk && checkoutRes.status === 201) {
    try {
      const order = JSON.parse(checkoutRes.body);
      if (order && order.order_id) {
        sleep(0.2);
        const pollRes = http.get(`${BASE_URL}/api/checkout/orders/${order.order_id}`, {
          tags: { name: '05_queue_poll' },
        });
        check(pollRes, { 'order poll ok': (r) => r.status === 200 });
      }
    } catch (e) {}
  }

  sleep(0.5 + Math.random() * 0.5);
}

export function handleSummary(data) {
  const p95 = data.metrics.http_req_duration ? data.metrics.http_req_duration.values['p(95)'].toFixed(2) : 'N/A';
  const failRate = data.metrics.http_req_failed ? (data.metrics.http_req_failed.values.rate * 100).toFixed(2) : '0.00';
  const totalReqs = data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0;

  console.log('\n========================================');
  console.log('SHOPKART QUEUE FLOOD LOAD RESULTS');
  console.log('========================================');
  console.log(`Total Requests Handled:  ${totalReqs}`);
  console.log(`P95 Latency:             ${p95} ms (Threshold: < 1200 ms)`);
  console.log(`HTTP Error Rate:         ${failRate}% (Threshold: < 10.0%)`);
  console.log('========================================\n');

  return { stdout: null };
}

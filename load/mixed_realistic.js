import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 15 },
    { duration: '1m30s', target: 30 }, // realistic mixed shopper traffic
    { duration: '30s', target: 5 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<800'],
    http_req_failed: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.GATEWAY_URL || 'http://localhost:9000';

const CATALOG_ITEMS = [
  { id: 'el_01', name: 'Sony WH-1000XM5 Wireless Noise Cancelling Headphones', price: 29990, q: 'sony' },
  { id: 'el_02', name: 'boAt Rockerz 450 Bluetooth On-Ear Headphones', price: 1499, q: 'boat' },
  { id: 'mb_01', name: 'Apple iPhone 15 (128 GB, Blue)', price: 71999, q: 'iphone' },
  { id: 'mb_04', name: 'OnePlus Nord CE 3 Lite 5G (Pastel Lime, 128 GB)', price: 19999, q: 'oneplus' },
  { id: 'fa_01', name: "Levi's Men 511 Slim Fit Stretchable Jeans", price: 2199, q: 'jeans' },
  { id: 'hk_01', name: 'Prestige Iris 750 Watt Mixer Grinder with 3 Jars', price: 3199, q: 'mixer' },
  { id: 'gr_01', name: 'Tata Tea Gold 1 kg Rich Taste and Aroma', price: 480, q: 'tea' },
  { id: 'sp_01', name: 'Boldfit Bold Rubber Hex Dumbbell 5kg Pair', price: 1899, q: 'dumbbell' },
];

const SHOPPERS = [
  { name: 'Aditi Rao', address: 'B-102, Shriram Spandhana, Wind Tunnel Road, Bengaluru 560037' },
  { name: 'Karthik Narayanan', address: 'Plot 88, Anna Nagar West, Chennai 600040' },
  { name: 'Siddharth Mehra', address: 'Flat 501, Raheja Vistas, NIBM Road, Pune 411048' },
  { name: 'Meenakshi Sundar', address: 'A-304, Godrej Platinum, Vikhroli, Mumbai 400079' },
];

export default function () {
  const item = CATALOG_ITEMS[Math.floor(Math.random() * CATALOG_ITEMS.length)];
  const shopper = SHOPPERS[Math.floor(Math.random() * SHOPPERS.length)];
  const cartId = `mix_cart_vu${__VU}_it${__ITER}_${Date.now().toString(36)}`;

  // Step 1: Browse / Search
  const searchRes = http.get(`${BASE_URL}/api/search/search?q=${item.q}`, {
    tags: { name: '01_mixed_search' },
  });
  check(searchRes, { 'search ok': (r) => r.status === 200 });

  // Realistic human think-time (1.0 - 2.5s)
  sleep(1.0 + Math.random() * 1.5);

  // Step 2: Product Detail & Verified Customer Reviews
  const detailRes = http.get(`${BASE_URL}/api/catalog/catalog/products/${item.id}`, {
    tags: { name: '02_mixed_detail' },
  });
  check(detailRes, { 'detail ok': (r) => r.status === 200 });

  const revRes = http.get(`${BASE_URL}/api/reviews/reviews/product/${item.id}`, {
    tags: { name: '02_mixed_reviews' },
  });
  check(revRes, { 'reviews ok': (r) => r.status === 200 });

  // Human think-time reading reviews (1.0 - 2.0s)
  sleep(1.0 + Math.random());

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
      tags: { name: '03_mixed_add_cart' },
    }
  );
  check(addRes, { 'cart ok': (r) => r.status === 200 });

  // Think-time deciding to purchase vs window-shop
  sleep(1.2 + Math.random() * 1.5);

  // Step 4: Checkout & Payment
  const checkoutRes = http.post(
    `${BASE_URL}/api/checkout/checkout`,
    JSON.stringify({
      cart_id: cartId,
      customer_name: shopper.name,
      customer_email: `${shopper.name.toLowerCase().replace(/\s+/g, '.')}@shopkart-shopper.in`,
      shipping_address: shopper.address,
      card_token: 'tok_visa_valid',
    }),
    {
      headers: { 'Content-Type': 'application/json' },
      tags: { name: '04_mixed_checkout' },
    }
  );

  const checkoutOk = check(checkoutRes, {
    'checkout handled': (r) => r.status === 201 || r.status === 503,
  });

  // Step 5: Order status polling
  if (checkoutOk && checkoutRes.status === 201) {
    try {
      const order = JSON.parse(checkoutRes.body);
      if (order && order.order_id) {
        sleep(0.5);
        const pollRes = http.get(`${BASE_URL}/api/checkout/orders/${order.order_id}`, {
          tags: { name: '05_mixed_poll' },
        });
        check(pollRes, { 'order poll ok': (r) => r.status === 200 });
      }
    } catch (e) {}
  }

  // Inter-journey pause (1.5 - 3.5s)
  sleep(1.5 + Math.random() * 2);
}

export function handleSummary(data) {
  const p95 = data.metrics.http_req_duration ? data.metrics.http_req_duration.values['p(95)'].toFixed(2) : 'N/A';
  const failRate = data.metrics.http_req_failed ? (data.metrics.http_req_failed.values.rate * 100).toFixed(2) : '0.00';
  const totalReqs = data.metrics.http_reqs ? data.metrics.http_reqs.values.count : 0;

  console.log('\n========================================');
  console.log('SHOPKART MIXED REALISTIC LOAD RESULTS');
  console.log('========================================');
  console.log(`Total Requests Handled:  ${totalReqs}`);
  console.log(`P95 Latency:             ${p95} ms (Threshold: < 800 ms)`);
  console.log(`HTTP Error Rate:         ${failRate}% (Threshold: < 5.0%)`);
  console.log('========================================\n');

  return { stdout: null };
}

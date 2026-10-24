# ShopKart Load & Fault Verification Results

> **Test Environment Notice**: All measurements recorded below were obtained in a controlled local performance test environment (Windows 11 x64, 16-core Intel i9 / 32GB RAM, Docker Engine 29.x with Linux Containers). These represent lab benchmark observations, not live customer production traffic.

---

## Baseline Load Profile (`load/baseline.js`)

- **Duration**: 80 seconds
- **Virtual Users**: Ramp from 5 to 10 VUs
- **Workload**: Catalog listing -> Keyword Search -> Cart Add -> Order Checkout
- **Observed Metrics**:
  - **Total HTTP Requests Completed**: 1,248
  - **Error Rate**: 0.00% (0 errors)
  - **Latency p50**: 28.4 ms
  - **Latency p95**: 118.2 ms
  - **Latency p99**: 184.6 ms
  - **Kafka Consumer Lag**: 0 (notification consumers keep pace with checkout orders)
  - **DB Pool Max In-Use**: 3 / 10 connections
- **Trace Status**: Complete end-to-end trace generated in Jaeger across `edge-gateway -> checkout -> inventory -> payments -> external-bank-mock -> kafka -> notifications`.

---

## Fault Scenario 1: Sudden Checkout Spike (`load/spike_checkout.js`)

- **Conditions**: 50 concurrent VUs hammering checkout with 1 checkout replica (default worker limit = 20).
- **Observed Behavior**:
  - Worker pool utilization reached 1.0 (100% capacity).
  - Excess requests returned `503 Service Unavailable` with `checkout service is at peak capacity`.
  - Inventory row-level lock contention elevated latency to ~450ms.
- **Scaling Remediation**:
  - Executed: `docker compose up -d --scale checkout=3 --no-recreate`.
  - Re-tested with 50 VUs: Throughput increased by 2.6x, 503 error rate dropped to 0%, and p95 latency decreased from 780ms to 92ms.

---

## Fault Scenario 2: Cache Outage (`scripts/fault_cache_down.sh`)

- **Observed Behavior**:
  - `cache_hits_total`: Flatlined to 0.
  - `cache_misses_total`: Increased by 48 req/s.
  - `db_pool_connections_in_use`: Rose from 2 to 9 connections on Postgres.
  - Cart operations failed with HTTP 500, correctly captured on storefront error banner.
- **Revert Verification**:
  - Executed `scripts/revert_cache_down.sh`.
  - Cache hits recovered to >95% within 15 seconds as queries repopulated Redis.

---

## Fault Scenario 3: Database Row Lock (`scripts/fault_db_lock.sh`)

- **Observed Behavior**:
  - An open transaction holding `SELECT ... FOR UPDATE` on `el_01` forced subsequent checkout requests to wait on lock acquisition.
  - `http_request_duration_seconds` for inventory reserve rose to the 5-second HTTP client timeout limit.
  - Requests for other unaffected items (e.g., `el_02`, `mb_04`) continued operating without latency impact, demonstrating genuine row-level lock isolation.
- **Revert Verification**:
  - Executed `scripts/revert_db_lock.sh`.
  - Blocking backend PID was terminated; backlog cleared within 1.2s.

---

## Fault Scenario 4: Connection Pool Exhaustion (`scripts/fault_slow_query.sh`)

- **Observed Behavior**:
  - `db_pool_connections_in_use`: Pinned at 10 (100%).
  - `db_pool_connections_wait_count`: Jumped from 0 to over 340 waits within 30 seconds.
  - Average catalog request latency degraded from 25ms to 2.4s.
- **Revert Verification**:
  - Executed `scripts/revert_slow_query.sh`.
  - Active queries terminated; pool in-use immediately fell back to 1.

---

## Fault Scenario 5: Leaky Memory Allocation (`scripts/fault_memory_pressure.sh`)

- **Observed Behavior**:
  - Memory consumption in `payments` climbed by ~20MB across 20 charge iterations.
  - `process_resident_memory_bytes` rose visibly in Grafana panel.
- **Revert Verification**:
  - Executed `scripts/revert_memory_pressure.sh`.
  - Memory cleared via admin endpoint; heap returned to baseline.

---

## Fault Scenario 6: Network Delay Injection (`scripts/fault_netem.sh`)

- **Observed Behavior**:
  - Injected 300ms base latency into external payment processor.
  - Jaeger traces showed `PaymentsApp.Charge` span widening by exactly ~330ms, with all internal database spans remaining under 5ms.
  - Storefront latency indicator transitioned from Green (`<100ms`) to Amber (`345ms Degraded`).
- **Revert Verification**:
  - Executed `scripts/revert_netem.sh`.
  - Trace duration returned to 35ms.

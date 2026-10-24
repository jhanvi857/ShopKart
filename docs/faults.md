# ShopKart Fault Injection & Incident Runbook

This document details the real fault mechanisms available in ShopKart, the observable symptoms across standard metrics and distributed traces, and the standard operational remediation a human SRE applies.

---

## 1. Unexpected Container Crash (`fault_crash.sh`)

### Injection & Revert
```bash
# Inject: Kill the inventory service
./scripts/fault_crash.sh shopkart-inventory

# Revert: Restart the container
./scripts/revert_crash.sh shopkart-inventory
```

### Observable Symptoms
- **Metrics (`http_requests_total`)**: Spikes in `502 Bad Gateway` from `edge-gateway` and `503 Service Unavailable` from `checkout`.
- **Traces (Jaeger)**: Spans for `CheckoutApp.Checkout` show error tags: `dial tcp: lookup inventory: connection refused`.
- **Storefront UX**: Add-to-cart or checkout actions display alert: *"inventory service unreachable"*.
- **cAdvisor / Docker**: Container status changes to `Exited`.

### Standard SRE Operational Fix
1. Inspect container logs: `docker logs shopkart-inventory`.
2. Verify host memory / OOM-killer status: `dmesg -T | grep -i oom`.
3. Restart or roll out replacement replica: `docker compose up -d --no-deps inventory`.

---

## 2. Complete Cache Outage (`fault_cache_down.sh`)

### Injection & Revert
```bash
# Inject: Stop Redis
./scripts/fault_cache_down.sh

# Revert: Restart Redis
./scripts/revert_cache_down.sh
```

### Observable Symptoms
- **Metrics (`cache_misses_total` / `cache_hits_total`)**: Cache hit rate drops immediately to 0%.
- **Metrics (`db_pool_connections_in_use`)**: Postgres connections spike as catalog falls back to database queries.
- **Cart Service (`/cart/*`)**: Returns `500 Internal Server Error` since Redis is primary persistence for ephemeral shopping carts.
- **Storefront UX**: Browsing catalog remains functional (slower), but cart actions fail with error banners.

### Standard SRE Operational Fix
1. Verify Redis process health: `docker logs shopkart-redis`.
2. Check Redis persistence file (RDB/AOF) integrity and restart Redis: `docker compose start redis`.
3. Monitor Redis reconnection logs in `catalog` and `cart`.

---

## 3. Database Row Lock Contention (`fault_db_lock.sh`)

### Injection & Revert
```bash
# Inject: Acquire exclusive row lock on el_01 (boAt Rockerz 550)
./scripts/fault_db_lock.sh

# Revert: Terminate blocking transaction
./scripts/revert_db_lock.sh
```

### Observable Symptoms
- **Metrics (`http_request_duration_seconds`)**: Latency for `POST /inventory/reserve` jumps from <10ms to transaction timeout (5s+).
- **Postgres Locks**: `SELECT * FROM pg_locks WHERE NOT granted;` shows waiting locks on the `inventory` table.
- **Traces (Jaeger)**: `InventoryApp.Reserve` span latency expands to multi-second duration.
- **Storefront UX**: Checkout button spins for 5s before timing out or throwing conflict error.

### Standard SRE Operational Fix
1. Identify the blocking PID in PostgreSQL:
   ```sql
   SELECT blocked_locks.pid AS blocked_pid, blocking_locks.pid AS blocking_pid, blocking_activity.query AS blocking_statement
   FROM pg_catalog.pg_locks blocked_locks
   JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
   JOIN pg_catalog.pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid AND blocking_locks.pid != blocked_locks.pid
   JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
   WHERE NOT blocked_locks.granted;
   ```
2. Terminate the blocking PID: `SELECT pg_cancel_backend(<blocking_pid>);` or `SELECT pg_terminate_backend(<blocking_pid>);`.

---

## 4. Connection Pool Saturation (`fault_slow_query.sh`)

### Injection & Revert
```bash
# Inject: Saturate pool with 12 parallel long-running queries
./scripts/fault_slow_query.sh

# Revert: Cancel the slow queries
./scripts/revert_slow_query.sh
```

### Observable Symptoms
- **Metrics (`db_pool_connections_in_use`)**: Pegged at maximum configured limit (10).
- **Metrics (`db_pool_connections_wait_count`)**: Monotonically increases rapidly.
- **Traces (Jaeger)**: DB spans report long wait times acquiring a connection from the pool.
- **Storefront UX**: Product queries and reviews take several seconds to load.

### Standard SRE Operational Fix
1. Dynamically increase DB connection pool limit via Admin API:
   ```bash
   curl -X POST http://localhost:9001/admin/config \
     -H "X-Admin-Key: $ADMIN_API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"db_max_open_conns": 25}'
   ```
2. Terminate rogue reporting / analytics queries on the transactional database replica.

---

## 5. Memory Pressure / Memory Leak (`fault_memory_pressure.sh`)

### Injection & Revert
```bash
# Inject: Activate genuine leak path in payments
./scripts/fault_memory_pressure.sh

# Revert: Disarm leak and free allocated buffers
./scripts/revert_memory_pressure.sh
```

### Observable Symptoms
- **Metrics (`process_resident_memory_bytes`)**: Memory climbs linearly per request until reaching container limit (256MB).
- **Metrics (`go_gc_duration_seconds`)**: GC frequency and pause times escalate sharply.
- **cAdvisor**: Container memory usage climbs toward 100% until Docker OOM killer terminates container if not reverted.

### Standard SRE Operational Fix
1. Inspect runtime memory statistics via `/admin/status`:
   ```bash
   curl -H "X-Admin-Key: $ADMIN_API_KEY" http://localhost:9005/admin/status
   ```
2. Take a Go pprof heap profile: `go tool pprof http://localhost:9005/debug/pprof/heap`.
3. Restart or rollback container and file critical bug report with payments engineering.

---

## 6. Network Latency & Jitter (`fault_netem.sh`)

### Injection & Revert
```bash
# Inject: Introduce 300ms latency to external-bank-mock
./scripts/fault_netem.sh shopkart-external-bank-mock 300ms 50ms

# Revert: Restore normal network latency
./scripts/revert_netem.sh shopkart-external-bank-mock
```

### Observable Symptoms
- **Metrics (`http_request_duration_seconds`)**: Payment p99 latency increases from 35ms to >350ms.
- **Traces (Jaeger)**: End-to-end trace clearly identifies `external-bank-mock` HTTP span as the sole latency bottleneck.
- **Storefront UX**: Footer latency indicator turns amber (`Degraded, ~380ms`).

### Standard SRE Operational Fix
1. Check synthetic monitoring / blackbox probes for external payment processor.
2. If third-party payment gateway is degraded, adjust client timeout or failover to secondary provider.

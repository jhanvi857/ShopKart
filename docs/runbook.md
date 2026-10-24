# ShopKart Operational Runbook

Standard operating procedures for managing, scaling, and reconfiguring the ShopKart production system.

---

## 1. Service Lifecycle Management

### Restarting a Service
```bash
# Restart without downtime to other services
docker compose restart <service-name>

# Example:
docker compose restart checkout
```

### Checking Service Health & Readiness
Every service implements standard liveness (`/healthz`) and dependency-aware readiness (`/readyz`) endpoints:
```bash
# Edge Gateway
curl http://localhost:9000/readyz

# Catalog
curl http://localhost:9001/readyz

# Checkout
curl http://localhost:9003/readyz

# Inventory
curl http://localhost:9004/readyz
```

---

## 2. Horizontal Scaling Behind Edge Gateway

The edge-gateway dynamically round-robins across all container instances registered under Docker DNS.

### Scaling Checkout Replicas
```bash
# Scale checkout to 3 instances
docker compose up -d --scale checkout=3 --no-recreate

# Verify container instances
docker compose ps checkout
```

### Scaling Catalog Replicas
```bash
# Scale catalog to 2 instances
docker compose up -d --scale catalog=2 --no-recreate
```

Under load tests (e.g., `k6 run load/spike_checkout.js`), scaling from 1 to 3 replicas increases overall throughput capacity by ~2.7x and eliminates 503 worker exhaustion errors.

---

## 3. Dynamic Runtime Reconfiguration (Admin API)

All services provide an authenticated admin endpoint protected by the shared secret `ADMIN_API_KEY`.

### Inspecting Runtime Stats
```bash
curl -s -H "X-Admin-Key: $ADMIN_API_KEY" http://localhost:9003/admin/status | jq .
```
Response includes:
- Active Go goroutine count
- Allocated memory (MB) and total GC cycles
- Current runtime settings

### Resizing Database Connection Pools at Runtime
When database pool exhaustion (`db_pool_connections_wait_count`) is observed:
```bash
# Increase DB pool in inventory
curl -s -X POST http://localhost:9004/admin/config \
  -H "X-Admin-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"db_max_open_conns": 20}'

# Increase DB pool in payments
curl -s -X POST http://localhost:9005/admin/config \
  -H "X-Admin-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"db_max_open_conns": 20}'
```

### Adjusting Kafka Consumer Concurrency
When consumer lag grows in notifications:
```bash
curl -s -X POST http://localhost:9007/admin/config \
  -H "X-Admin-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"notification_workers": 8, "process_delay_ms": 10}'
```

### Flusing Application Cache
When product prices or descriptions are updated in Postgres and require immediate cache invalidation:
```bash
curl -s -X POST http://localhost:9001/admin/cache/flush \
  -H "X-Admin-Key: $ADMIN_API_KEY"
```

---

## 4. Observability Quick Reference

| Tool | URL | Credentials | Purpose |
| :--- | :--- | :--- | :--- |
| **Grafana** | `http://localhost:3001` | `admin` / `admin` | Real-time Golden Signals Dashboard |
| **Prometheus** | `http://localhost:9091` | None | Raw PromQL metric queries |
| **Jaeger** | `http://localhost:16686` | None | Distributed Traces across HTTP & Kafka |
| **Storefront** | `http://localhost:3000` | None | Customer Web App & Latency Indicator |

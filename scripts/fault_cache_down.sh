#!/usr/bin/env bash
# fault_cache_down.sh: Stops Redis container to simulate complete cache tier outage
set -euo pipefail

echo "[FAULT] Stopping Redis cache tier (shopkart-redis)..."
docker stop shopkart-redis || true
echo "[FAULT] Redis stopped. Catalog and Cart services will report degraded cache operations."

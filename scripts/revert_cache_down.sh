#!/usr/bin/env bash
# revert_cache_down.sh: Starts Redis container to restore cache tier
set -euo pipefail

echo "[REVERT] Starting Redis cache tier (shopkart-redis)..."
docker start shopkart-redis || docker compose start redis
echo "[REVERT] Redis cache is back online."

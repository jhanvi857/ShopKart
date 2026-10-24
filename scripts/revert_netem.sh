#!/usr/bin/env bash
# revert_netem.sh: Removes network latency emulation
set -euo pipefail

TARGET="${1:-shopkart-external-bank-mock}"

echo "[REVERT] Reverting network delay on ${TARGET}..."

docker exec --privileged "${TARGET}" tc qdisc del dev eth0 root 2>/dev/null || true
curl -s -X POST "http://localhost:9008/admin/config" \
  -H "Content-Type: application/json" \
  -d "{\"base_latency_ms\": 35, \"jitter_ms\": 15}" > /dev/null || true

echo "[REVERT] Network delay reverted to normal baseline (35ms)."

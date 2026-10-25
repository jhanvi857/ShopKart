#!/usr/bin/env bash
# revert_netem.sh: Removes network latency emulation
set -euo pipefail
if command -v curl.exe >/dev/null 2>&1; then CURL_BIN="curl.exe"; else CURL_BIN="curl"; fi

TARGET="${1:-shopkart-external-bank-mock}"

echo "[REVERT] Reverting network delay on ${TARGET}..."

docker exec --privileged "${TARGET}" tc qdisc del dev eth0 root 2>/dev/null || true
"${CURL_BIN}" -fsS -X POST "http://127.0.0.1:9008/admin/config" \
  -H "Content-Type: application/json" \
  -d "{\"base_latency_ms\": 35, \"jitter_ms\": 15}" > /dev/null

echo "[REVERT] Network delay reverted to normal baseline (35ms)."

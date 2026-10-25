#!/usr/bin/env bash
# fault_netem.sh: Injects network latency and packet jitter into edge services
set -euo pipefail
if command -v curl.exe >/dev/null 2>&1; then CURL_BIN="curl.exe"; else CURL_BIN="curl"; fi

TARGET="${1:-shopkart-external-bank-mock}"
LATENCY="${2:-300ms}"
JITTER="${3:-50ms}"

echo "[FAULT] Injecting network delay (${LATENCY} +/- ${JITTER}) on ${TARGET}..."

# 1. Attempt Linux tc netem in container
if docker exec --privileged "${TARGET}" tc qdisc replace dev eth0 root netem delay "${LATENCY}" "${JITTER}" 2>/dev/null; then
  echo "[FAULT] Applied tc netem rule on ${TARGET} eth0 interface."
else
  # 2. Fallback to application-layer link delay via admin endpoint if tc not permitted
  echo "[FAULT] Container lacking NET_ADMIN capability; configuring mock upstream latency via admin endpoint..."
  "${CURL_BIN}" -fsS -X POST "http://127.0.0.1:9008/admin/config" \
    -H "Content-Type: application/json" \
    -d "{\"base_latency_ms\": 350, \"jitter_ms\": 75}" > /dev/null
  echo "[FAULT] Injected 350ms link latency via bank mock."
fi

#!/usr/bin/env bash
# revert_memory_pressure.sh: Disables and clears the memory leak in payments service
set -euo pipefail

PAYMENTS_URL="${PAYMENTS_URL:-http://127.0.0.1:9005}"
: "${ADMIN_API_KEY:?ADMIN_API_KEY must be set in the environment}"
ADMIN_KEY="${ADMIN_API_KEY}"
if command -v curl.exe >/dev/null 2>&1; then CURL_BIN="curl.exe"; else CURL_BIN="curl"; fi

echo "[REVERT] Disabling memory leak path..."
"${CURL_BIN}" -fsS -X POST "${PAYMENTS_URL}/admin/fault/leak-disable" \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H "Content-Type: application/json"

echo -e "\n[REVERT] Clearing allocated memory buffers..."
"${CURL_BIN}" -fsS -X POST "${PAYMENTS_URL}/admin/fault/leak-clear" \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H "Content-Type: application/json"

echo -e "\n[REVERT] Memory leak neutralized and memory returned."

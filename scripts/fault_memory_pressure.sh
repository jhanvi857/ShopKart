#!/usr/bin/env bash
# fault_memory_pressure.sh: Activates the genuine leaky code path in payments service
set -euo pipefail

PAYMENTS_URL="${PAYMENTS_URL:-http://127.0.0.1:9005}"
: "${ADMIN_API_KEY:?ADMIN_API_KEY must be set in the environment}"
ADMIN_KEY="${ADMIN_API_KEY}"
if command -v curl.exe >/dev/null 2>&1; then CURL_BIN="curl.exe"; else CURL_BIN="curl"; fi

echo "[FAULT] Enabling genuine memory leak in payments service..."
"${CURL_BIN}" -fsS -X POST "${PAYMENTS_URL}/admin/fault/leak-enable" \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H "Content-Type: application/json"

echo -e "\n[FAULT] Leaky path enabled. Exercising payment allocations to induce memory pressure..."
for i in {1..20}; do
  "${CURL_BIN}" -fsS -X POST "${PAYMENTS_URL}/payments/charge" \
    -H "Content-Type: application/json" \
    -d "{\"order_id\":\"ord_leak_${i}\",\"amount\":10.0,\"currency\":\"USD\",\"idempotency_key\":\"ik_leak_${i}\",\"card_token\":\"tok_leak\"}" > /dev/null
done

echo "[FAULT] Memory allocations complete. Check Prometheus process_resident_memory_bytes."

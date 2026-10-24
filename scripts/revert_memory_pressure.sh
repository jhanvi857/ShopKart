#!/usr/bin/env bash
# revert_memory_pressure.sh: Disables and clears the memory leak in payments service
set -euo pipefail

PAYMENTS_URL="${PAYMENTS_URL:-http://localhost:9005}"
ADMIN_KEY="${ADMIN_API_KEY:-shopkart-ops-secret-key-prod-998}"

echo "[REVERT] Disabling memory leak path..."
curl -s -X POST "${PAYMENTS_URL}/admin/fault/leak-disable" \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H "Content-Type: application/json"

echo -e "\n[REVERT] Clearing allocated memory buffers..."
curl -s -X POST "${PAYMENTS_URL}/admin/fault/leak-clear" \
  -H "X-Admin-Key: ${ADMIN_KEY}" \
  -H "Content-Type: application/json"

echo -e "\n[REVERT] Memory leak neutralized and memory returned."

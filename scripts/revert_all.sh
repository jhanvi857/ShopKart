#!/usr/bin/env bash
# revert_all.sh: Reverts all active faults and restores clean baseline
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== Restoring ShopKart System to Healthy Baseline ==="

echo "1. Reverting crashed containers..."
"${SCRIPT_DIR}/revert_crash.sh" "inventory" "9004" || true

echo "2. Reverting cache tier..."
"${SCRIPT_DIR}/revert_cache_down.sh" || true

echo "3. Releasing DB row locks..."
"${SCRIPT_DIR}/revert_db_lock.sh" || true

echo "4. Terminating slow queries and clearing connection pool..."
"${SCRIPT_DIR}/revert_slow_query.sh" || true

echo "5. Neutralizing memory leak..."
"${SCRIPT_DIR}/revert_memory_pressure.sh" || true

echo "6. Resetting network latency..."
"${SCRIPT_DIR}/revert_netem.sh" || true

echo "=== All Faults Reverted. System Baseline Restored ==="

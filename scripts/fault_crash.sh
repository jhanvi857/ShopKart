#!/usr/bin/env bash
# fault_crash.sh: Kills a specified service container to simulate a sudden process crash
set -euo pipefail

TARGET="${1:-shopkart-inventory}"
echo "[FAULT] Simulating unexpected container crash on ${TARGET}..."

if docker ps --format '{{.Names}}' | grep -q "^${TARGET}$"; then
  docker kill "${TARGET}"
  echo "[FAULT] Successfully killed ${TARGET}. Dependent services will experience connection refused/timeouts."
else
  echo "[FAULT] Container ${TARGET} is already not running."
fi

#!/usr/bin/env bash
# revert_crash.sh: Restarts a crashed container
set -euo pipefail

TARGET="${1:-shopkart-inventory}"
echo "[REVERT] Recovering ${TARGET}..."

docker start "${TARGET}" || docker compose start "${TARGET#shopkart-}"
echo "[REVERT] Service ${TARGET} restarted and operational."

#!/usr/bin/env bash
# fault_crash.sh: Kills a native Windows process identified by its listening port.
set -euo pipefail

TARGET_PORT="${1:-9004}"
echo "[FAULT] Simulating unexpected native process crash on port ${TARGET_PORT}..."

PID="$(powershell.exe -NoProfile -Command "(Get-NetTCPConnection -LocalPort ${TARGET_PORT} -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty OwningProcess)")"
if [ -z "${PID}" ]; then
  echo "[FAULT] No process is listening on port ${TARGET_PORT}." >&2
  exit 1
fi
powershell.exe -NoProfile -Command "Stop-Process -Id ${PID}"
echo "[FAULT] Successfully killed native process ${PID} on port ${TARGET_PORT}."

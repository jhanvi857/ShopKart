#!/usr/bin/env bash
# revert_crash.sh: Restarts a native Go service after a targeted process crash.
set -euo pipefail

SERVICE="${1:-inventory}"
PORT="${2:-9004}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
LOG_DIR="${ROOT_DIR}/.fault-logs"
mkdir -p "${LOG_DIR}"
ROOT_WIN="$(wslpath -w "${ROOT_DIR}")"
LOG_DIR_WIN="$(wslpath -w "${LOG_DIR}")"
echo "[REVERT] Recovering native ${SERVICE} on port ${PORT}..."

if powershell.exe -NoProfile -Command "(Get-NetTCPConnection -LocalPort ${PORT} -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty OwningProcess)" | grep -q '[0-9]'; then
  echo "[REVERT] Port ${PORT} is already listening."
  exit 0
fi
powershell.exe -NoProfile -Command "Start-Process -FilePath 'go' -ArgumentList 'run','./${SERVICE}' -WorkingDirectory '${ROOT_WIN}\\services' -RedirectStandardOutput '${LOG_DIR_WIN}\\${SERVICE}.out.log' -RedirectStandardError '${LOG_DIR_WIN}\\${SERVICE}.err.log'"
for i in {1..30}; do
  if powershell.exe -NoProfile -Command "(Get-NetTCPConnection -LocalPort ${PORT} -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty OwningProcess)" | grep -q '[0-9]'; then
    echo "[REVERT] Service ${SERVICE} is listening on port ${PORT}."
    exit 0
  fi
  sleep 1
done
echo "[REVERT] Service ${SERVICE} did not become ready. Check ${LOG_DIR}/${SERVICE}.err.log." >&2
exit 1

#!/usr/bin/env bash
# revert_db_lock.sh: Terminates any blocking queries or sleep transactions on Postgres
set -euo pipefail

DB_USER="${POSTGRES_USER:-shopkart_user}"
DB_NAME="${POSTGRES_DB:-shopkart_db}"

echo "[REVERT] Terminating blocking backend transactions on Postgres..."

docker exec shopkart-postgres psql -U "${DB_USER}" -d "${DB_NAME}" -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE query LIKE '%pg_sleep%' AND pid <> pg_backend_pid();"

echo "[REVERT] Blocking transactions terminated. Row locks released."

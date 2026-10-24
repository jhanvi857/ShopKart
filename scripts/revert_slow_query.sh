#!/usr/bin/env bash
# revert_slow_query.sh: Cancels and terminates long-running queries
set -euo pipefail

DB_USER="${POSTGRES_USER:-shopkart_user}"
DB_NAME="${POSTGRES_DB:-shopkart_db}"

echo "[REVERT] Terminating slow queries and releasing DB connection pool..."

docker exec shopkart-postgres psql -U "${DB_USER}" -d "${DB_NAME}" -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'active' AND query LIKE '%generate_series%' AND pid <> pg_backend_pid();"

echo "[REVERT] Connection pool cleared and restored."

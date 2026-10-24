#!/usr/bin/env bash
# fault_slow_query.sh: Exhausts Postgres connection pool by spawning persistent heavy queries
set -euo pipefail

DB_USER="${POSTGRES_USER:-shopkart_user}"
DB_NAME="${POSTGRES_DB:-shopkart_db}"

echo "[FAULT] Spawning 12 slow query sessions on Postgres to saturate connection pool..."

for i in {1..12}; do
  docker exec -d shopkart-postgres psql -U "${DB_USER}" -d "${DB_NAME}" -c \
    "SELECT pg_sleep(120), count(*) FROM generate_series(1, 5000000);"
done

echo "[FAULT] 12 slow queries running. Database pool wait counts and latency will escalate."

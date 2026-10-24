#!/usr/bin/env bash
# fault_db_lock.sh: Holds an exclusive row lock on hot inventory items using a real background transaction
set -euo pipefail

DB_USER="${POSTGRES_USER:-shopkart_user}"
DB_NAME="${POSTGRES_DB:-shopkart_db}"

echo "[FAULT] Injecting long row-level lock on inventory (el_01) in Postgres..."

# Run an explicit transaction with pg_sleep to block stock decrements
docker exec -d shopkart-postgres psql -U "${DB_USER}" -d "${DB_NAME}" -c \
  "BEGIN; SELECT * FROM inventory WHERE product_id = 'el_01' FOR UPDATE; SELECT pg_sleep(300); COMMIT;"

echo "[FAULT] Row lock acquired on el_01 for 300s. Checkouts for el_01 will now queue and block."

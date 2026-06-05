#!/bin/sh
set -e

PGDATA="/var/lib/postgresql/data"

# Fresh install: no data directory yet, skip password sync
if [ ! -f "$PGDATA/PG_VERSION" ]; then
  echo "Fresh install detected, skipping password sync"
  exit 0
fi

echo "Existing database detected, syncing passwords..."

gosu postgres postgres --single -D "$PGDATA" postgres <<SQL
ALTER ROLE postgres WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE supabase_admin WITH PASSWORD '${PGPASSWORD}';
SQL

echo "Password sync completed"

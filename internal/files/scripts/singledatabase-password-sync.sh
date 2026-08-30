#!/bin/sh
set -e

PGDATA="/var/lib/postgresql/data"

# Fresh install: no data directory yet, skip password sync
if [ ! -f "$PGDATA/PG_VERSION" ]; then
  echo "Fresh install detected, skipping password sync"
  exit 0
fi

echo "Existing database detected, syncing passwords..."

# postgres --single does not support psql's -v variable binding,
# so we escape single quotes in the password (doubling them) to
# prevent SQL injection if the value ever contains a single quote.
ESCAPED_PGPASSWORD=$(printf '%s' "$PGPASSWORD" | sed "s/'/''/g")

gosu postgres postgres --single -D "$PGDATA" postgres <<SQL
ALTER ROLE postgres WITH PASSWORD '${ESCAPED_PGPASSWORD}';
ALTER ROLE supabase_admin WITH PASSWORD '${ESCAPED_PGPASSWORD}';
SQL

echo "Password sync completed"

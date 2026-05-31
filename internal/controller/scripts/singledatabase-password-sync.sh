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
ALTER ROLE anon WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE authenticated WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE authenticator WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE dashboard_user WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE pgbouncer WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE postgres WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE service_role WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE supabase_admin WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE supabase_auth_admin WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE supabase_functions_admin WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE supabase_replication_admin WITH PASSWORD '${PGPASSWORD}';
ALTER ROLE supabase_storage_admin WITH PASSWORD '${PGPASSWORD}';
DROP SCHEMA IF EXISTS _supavisor CASCADE;
CREATE SCHEMA IF NOT EXISTS _supavisor;
ALTER SCHEMA _supavisor OWNER TO supabase_admin;
SQL

echo "Password sync completed"

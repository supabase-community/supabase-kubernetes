#!/bin/sh
set -e

# Wait for database to be ready
until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"; do
  echo "Waiting for database..."
  sleep 2
done

# Apply password sync via psql remote connection
psql -v ON_ERROR_STOP=1 -c "
ALTER USER anon WITH PASSWORD '${PGPASSWORD}';
ALTER USER authenticated WITH PASSWORD '${PGPASSWORD}';
ALTER USER authenticator WITH PASSWORD '${PGPASSWORD}';
ALTER USER dashboard_user WITH PASSWORD '${PGPASSWORD}';
ALTER USER pgbouncer WITH PASSWORD '${PGPASSWORD}';
ALTER USER postgres WITH PASSWORD '${PGPASSWORD}';
ALTER USER service_role WITH PASSWORD '${PGPASSWORD}';
ALTER USER supabase_admin WITH PASSWORD '${PGPASSWORD}';
ALTER USER supabase_auth_admin WITH PASSWORD '${PGPASSWORD}';
ALTER USER supabase_functions_admin WITH PASSWORD '${PGPASSWORD}';
ALTER USER supabase_replication_admin WITH PASSWORD '${PGPASSWORD}';
ALTER USER supabase_storage_admin WITH PASSWORD '${PGPASSWORD}';

DROP SCHEMA IF EXISTS _supavisor CASCADE;
CREATE SCHEMA IF NOT EXISTS _supavisor;
ALTER SCHEMA _supavisor OWNER TO supabase_admin;

DO \$\$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.tables
    WHERE table_schema = '_analytics'
      AND table_name = 'source_backends'
  ) THEN
    UPDATE _analytics.source_backends
    SET config = jsonb_set(
      config,
      '{url}',
      '\"postgresql://${DB_ADMIN_USER}:${PGPASSWORD}@${DB_SRV_NAME}:${DB_SRV_PORT}/postgres\"',
      false
    )
    WHERE type = 'postgres';
  END IF;
END
\$\$;
"

echo "Password sync completed successfully"

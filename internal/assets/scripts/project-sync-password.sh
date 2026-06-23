#!/bin/sh
set -e

# Wait for database to be ready
until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"; do
  echo "Waiting for database..."
  sleep 2
done

# Apply password sync via psql remote connection.
# Use psql -v variable binding (:'var') to safely quote values,
# preventing SQL injection if values ever contain single quotes.
# The heredoc uses a single-quoted delimiter (<<'EOSQL') so the shell
# does not expand variables -- psql handles all substitution.
DB_URL="postgresql://${DB_ADMIN_USER}:${PGPASSWORD}@${DB_SRV_NAME}:${DB_SRV_PORT}/postgres"

psql -v ON_ERROR_STOP=1 \
     -v pgpassword="$PGPASSWORD" \
     -v db_url="$DB_URL" <<'EOSQL'
ALTER USER anon WITH PASSWORD :'pgpassword';
ALTER USER authenticated WITH PASSWORD :'pgpassword';
ALTER USER authenticator WITH PASSWORD :'pgpassword';
ALTER USER dashboard_user WITH PASSWORD :'pgpassword';
ALTER USER pgbouncer WITH PASSWORD :'pgpassword';
ALTER USER postgres WITH PASSWORD :'pgpassword';
ALTER USER service_role WITH PASSWORD :'pgpassword';
ALTER USER supabase_admin WITH PASSWORD :'pgpassword';
ALTER USER supabase_auth_admin WITH PASSWORD :'pgpassword';
ALTER USER supabase_functions_admin WITH PASSWORD :'pgpassword';
ALTER USER supabase_replication_admin WITH PASSWORD :'pgpassword';
ALTER USER supabase_storage_admin WITH PASSWORD :'pgpassword';

DROP SCHEMA IF EXISTS _supavisor CASCADE;
CREATE SCHEMA IF NOT EXISTS _supavisor;
ALTER SCHEMA _supavisor OWNER TO supabase_admin;

-- Pass db_url into a session GUC so PL/pgSQL can read it via
-- current_setting(). psql does not interpolate :'var' inside
-- dollar-quoted PL/pgSQL blocks, so this is the bridge.
SET myapp.db_url = :'db_url';

DO $fn$
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
      to_jsonb(current_setting('myapp.db_url')),
      false
    )
    WHERE type = 'postgres';
  END IF;
END
$fn$;

-- Clean up the session GUC
RESET myapp.db_url;
EOSQL

echo "Password sync completed successfully"

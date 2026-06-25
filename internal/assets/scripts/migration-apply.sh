#!/bin/sh
set -e

# Wait for database to be ready
until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"; do
  echo "Waiting for database..."
  sleep 2
done

# Create operator schema and migrations tracking table if not exists.
# Use psql -v variable binding to safely interpolate values.
# Variable substitution (:var, :'var') only works when psql reads from
# stdin (heredoc) or -f files, NOT with -c. So we use heredocs throughout.
# :migration_table is used as an unquoted identifier substitution,
# :'migration_hash' is used as a safely-quoted string literal.
psql -v ON_ERROR_STOP=1 \
     -v migration_table="$MIGRATION_TABLE" <<'EOSQL'
CREATE SCHEMA IF NOT EXISTS supabase_operator;
CREATE TABLE IF NOT EXISTS :migration_table (hash TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
EOSQL

# Check if already applied (idempotency at DB level)
ALREADY_APPLIED=$(psql -v ON_ERROR_STOP=1 -tA \
     -v migration_table="$MIGRATION_TABLE" \
     -v migration_hash="$MIGRATION_HASH" <<'EOSQL'
SELECT 1 FROM :migration_table WHERE hash = :'migration_hash';
EOSQL
)

if [ "$ALREADY_APPLIED" = "1" ]; then
    echo "Migration batch already applied, skipping"
    exit 0
fi

# Apply migration batch atomically
psql -v ON_ERROR_STOP=1 -f "$MIGRATION_BATCH_PATH"

# Record applied hash
psql -v ON_ERROR_STOP=1 \
     -v migration_table="$MIGRATION_TABLE" \
     -v migration_hash="$MIGRATION_HASH" <<'EOSQL'
INSERT INTO :migration_table (hash) VALUES (:'migration_hash');
EOSQL

echo "Migration batch applied successfully"

#!/bin/sh
set -e

# Wait for database to be ready
until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"; do
  echo "Waiting for database..."
  sleep 2
done

# Create operator schema and migrations tracking table if not exists
psql -v ON_ERROR_STOP=1 -c "CREATE SCHEMA IF NOT EXISTS supabase_operator;"
psql -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS $MIGRATION_TABLE (hash TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW());"

# Check if already applied (idempotency at DB level)
ALREADY_APPLIED=$(psql -v ON_ERROR_STOP=1 -tAc "SELECT 1 FROM $MIGRATION_TABLE WHERE hash = '$MIGRATION_HASH';")

if [ "$ALREADY_APPLIED" = "1" ]; then
    echo "Migration batch already applied, skipping"
    exit 0
fi

# Apply migration batch atomically
psql -v ON_ERROR_STOP=1 -f "$MIGRATION_BATCH_PATH"

# Record applied hash
psql -v ON_ERROR_STOP=1 -c "INSERT INTO $MIGRATION_TABLE (hash) VALUES ('$MIGRATION_HASH');"

echo "Migration batch applied successfully"

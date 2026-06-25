#!/bin/sh
set -e

# Wait for database to be ready
until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"; do
  echo "Waiting for database..."
  sleep 2
done

# Apply JWT settings
# Use psql -v variable binding (:'var') to safely quote values,
# preventing SQL injection if values ever contain single quotes.
psql -v ON_ERROR_STOP=1 \
     -v jwt_secret="$JWT_SECRET" \
     -v jwt_exp="$JWT_EXP" <<'EOSQL'
ALTER DATABASE postgres SET "app.settings.jwt_secret" TO :'jwt_secret';
ALTER DATABASE postgres SET "app.settings.jwt_exp" TO :'jwt_exp';
EOSQL

echo "JWT settings updated successfully"

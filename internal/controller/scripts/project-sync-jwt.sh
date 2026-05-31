#!/bin/sh
set -e

# Wait for database to be ready
until pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER"; do
  echo "Waiting for database..."
  sleep 2
done

# Apply JWT settings
psql -v ON_ERROR_STOP=1 -c "
ALTER DATABASE postgres SET \"app.settings.jwt_secret\" TO '${JWT_SECRET}';
ALTER DATABASE postgres SET \"app.settings.jwt_exp\" TO '${JWT_EXP}';
"

echo "JWT settings updated successfully"

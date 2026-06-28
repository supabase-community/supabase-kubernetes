SELECT format('CREATE DATABASE %I WITH OWNER %I', :'internal_db_name', :'service_owner')
WHERE NOT EXISTS (
  SELECT 1
  FROM pg_database
  WHERE datname = :'internal_db_name'
) \gexec

\connect :internal_db_name

CREATE SCHEMA IF NOT EXISTS _analytics;
SELECT format('ALTER SCHEMA _analytics OWNER TO %I', :'service_owner') \gexec

CREATE SCHEMA IF NOT EXISTS _supavisor;
SELECT format('ALTER SCHEMA _supavisor OWNER TO %I', :'service_owner') \gexec

\connect :db_name

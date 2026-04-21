-- Supabase initial schema: roles, extensions, schemas, and default grants.
-- Adapted from supabase/postgres init-scripts for external (managed) databases
-- such as AWS RDS, Google Cloud SQL, Azure Database, etc.
--
-- Key differences from the internal DB init:
--   - All role creation is idempotent (IF NOT EXISTS)
--   - REPLICATION attribute skipped (not available on most managed databases)
--   - No reference to 'postgres' role (RDS uses a custom master user)
--   - Role passwords set from the DB master password
--   - graphql_public schema created for PostgREST
--   - All auth functions owned by supabase_auth_admin

-- Extension namespacing
CREATE SCHEMA IF NOT EXISTS extensions;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA extensions;
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions;

-- Compatibility roles: ensure both 'postgres' and 'supabase_admin' exist regardless
-- of which was used as the master username. Supabase services connect as supabase_admin
-- and upstream migrations reference postgres in grants.
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'postgres') THEN CREATE ROLE postgres NOLOGIN; END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_admin') THEN CREATE USER supabase_admin CREATEDB CREATEROLE LOGIN BYPASSRLS; END IF; END $$;

-- Core API roles
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'anon') THEN CREATE ROLE anon NOLOGIN NOINHERIT; END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'authenticated') THEN CREATE ROLE authenticated NOLOGIN NOINHERIT; END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'service_role') THEN CREATE ROLE service_role NOLOGIN NOINHERIT BYPASSRLS; END IF; END $$;

-- Service users
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'authenticator') THEN CREATE USER authenticator NOINHERIT; END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_auth_admin') THEN CREATE USER supabase_auth_admin NOINHERIT CREATEROLE LOGIN NOREPLICATION; END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'supabase_storage_admin') THEN CREATE USER supabase_storage_admin NOINHERIT CREATEROLE LOGIN NOREPLICATION; END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'pgbouncer') THEN CREATE USER pgbouncer; END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'dashboard_user') THEN CREATE ROLE dashboard_user NOSUPERUSER CREATEDB CREATEROLE; END IF; END $$;

-- Grant API roles to authenticator
GRANT anon TO authenticator;
GRANT authenticated TO authenticator;
GRANT service_role TO authenticator;

-- Public schema grants
GRANT USAGE ON SCHEMA public TO postgres, anon, authenticated, service_role;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO postgres, anon, authenticated, service_role;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO postgres, anon, authenticated, service_role;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO postgres, anon, authenticated, service_role;
GRANT USAGE ON SCHEMA extensions TO postgres, anon, authenticated, service_role;

-- API role timeouts
ALTER ROLE anon SET statement_timeout = '3s';
ALTER ROLE authenticated SET statement_timeout = '8s';

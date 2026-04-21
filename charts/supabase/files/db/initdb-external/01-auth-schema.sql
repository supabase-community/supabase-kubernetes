-- Auth schema, supabase_auth_admin role, and auth helper functions
-- Grant membership so current user can SET ROLE
GRANT supabase_auth_admin TO CURRENT_USER;

CREATE SCHEMA IF NOT EXISTS auth;
ALTER SCHEMA auth OWNER TO supabase_auth_admin;
GRANT ALL PRIVILEGES ON SCHEMA auth TO supabase_auth_admin;
ALTER USER supabase_auth_admin SET search_path = 'auth';
GRANT USAGE ON SCHEMA auth TO anon, authenticated, service_role;

-- Auth helper functions (owned by supabase_auth_admin to avoid permission issues)
SET ROLE supabase_auth_admin;

CREATE OR REPLACE FUNCTION auth.uid() RETURNS uuid AS $$
  SELECT nullif(current_setting('request.jwt.claim.sub', true), '')::uuid;
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION auth.role() RETURNS text AS $$
  SELECT nullif(current_setting('request.jwt.claim.role', true), '')::text;
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION auth.email() RETURNS text AS $$
  SELECT nullif(current_setting('request.jwt.claim.email', true), '')::text;
$$ LANGUAGE sql STABLE;

RESET ROLE;

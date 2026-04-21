-- Storage schema and supabase_storage_admin role
-- Grant membership so current user can SET ROLE
GRANT supabase_storage_admin TO CURRENT_USER;

CREATE SCHEMA IF NOT EXISTS storage;
ALTER SCHEMA storage OWNER TO supabase_storage_admin;
ALTER USER supabase_storage_admin SET search_path = 'storage';
GRANT USAGE ON SCHEMA storage TO postgres, anon, authenticated, service_role;
ALTER DEFAULT PRIVILEGES IN SCHEMA storage GRANT ALL ON TABLES TO postgres, anon, authenticated, service_role;
ALTER DEFAULT PRIVILEGES IN SCHEMA storage GRANT ALL ON FUNCTIONS TO postgres, anon, authenticated, service_role;
ALTER DEFAULT PRIVILEGES IN SCHEMA storage GRANT ALL ON SEQUENCES TO postgres, anon, authenticated, service_role;
GRANT ALL ON SCHEMA storage TO supabase_storage_admin WITH GRANT OPTION;

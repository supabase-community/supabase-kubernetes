SELECT format('ALTER ROLE supabase_admin SET default_table_access_method = %I', :'default_table_access_method')
WHERE EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'supabase_admin') \gexec

SELECT format('ALTER ROLE supabase_auth_admin SET default_table_access_method = %I', :'default_table_access_method')
WHERE EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'supabase_auth_admin') \gexec

SELECT format('ALTER ROLE supabase_functions_admin SET default_table_access_method = %I', :'default_table_access_method')
WHERE EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'supabase_functions_admin') \gexec

SELECT format('ALTER ROLE supabase_storage_admin SET default_table_access_method = %I', :'default_table_access_method')
WHERE EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'supabase_storage_admin') \gexec

SELECT format('ALTER DATABASE _supabase SET default_table_access_method = %I', :'default_table_access_method')
WHERE EXISTS (
  SELECT 1
  FROM pg_database
  WHERE datname = '_supabase'
) \gexec

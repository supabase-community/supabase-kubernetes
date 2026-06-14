CREATE SCHEMA IF NOT EXISTS extensions;
SET default_table_access_method = heap;
CREATE EXTENSION IF NOT EXISTS orioledb WITH SCHEMA extensions;
RESET default_table_access_method;

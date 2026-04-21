-- Dashboard user grants
DO $$ BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'dashboard_user') THEN
    GRANT ALL ON SCHEMA auth TO dashboard_user;
    GRANT ALL ON SCHEMA extensions TO dashboard_user;
    GRANT ALL ON ALL TABLES IN SCHEMA auth TO dashboard_user;
    GRANT ALL ON ALL TABLES IN SCHEMA extensions TO dashboard_user;
    GRANT ALL ON ALL SEQUENCES IN SCHEMA auth TO dashboard_user;
    GRANT ALL ON ALL SEQUENCES IN SCHEMA extensions TO dashboard_user;
    GRANT ALL ON ALL ROUTINES IN SCHEMA auth TO dashboard_user;
    GRANT ALL ON ALL ROUTINES IN SCHEMA extensions TO dashboard_user;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'dashboard_user') AND
     EXISTS (SELECT FROM pg_namespace WHERE nspname = 'storage') THEN
    GRANT ALL ON SCHEMA storage TO dashboard_user;
    GRANT ALL ON ALL SEQUENCES IN SCHEMA storage TO dashboard_user;
    GRANT ALL ON ALL ROUTINES IN SCHEMA storage TO dashboard_user;
  END IF;
END $$;

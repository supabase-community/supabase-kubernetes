SELECT format('ALTER DATABASE %I SET "app.settings.jwt_secret" TO %L', :'db_name', :'jwt_secret') \gexec
SELECT format('ALTER DATABASE %I SET "app.settings.jwt_exp" TO %L', :'db_name', :'jwt_exp') \gexec

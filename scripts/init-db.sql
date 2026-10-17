-- Development credentials, bound to loopback by Compose.
CREATE ROLE compass_app LOGIN PASSWORD 'compass_app_local';
GRANT CONNECT ON DATABASE compass TO compass_app;
GRANT USAGE ON SCHEMA public TO compass_app;
ALTER DEFAULT PRIVILEGES FOR ROLE compass IN SCHEMA public GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO compass_app;

-- Rollback: drop users and auth_settings tables.
-- WARNING: this will destroy data. Only use in development/testing.

DROP TABLE IF EXISTS auth_settings;
DROP TABLE IF EXISTS users;

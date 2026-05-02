DROP INDEX IF EXISTS idx_audit_log_ts;
DROP TABLE IF EXISTS audit_log;

DROP INDEX IF EXISTS idx_sessions_expires_at;
DROP INDEX IF EXISTS idx_sessions_username;
DROP TABLE IF EXISTS sessions;

DROP TABLE IF EXISTS users;

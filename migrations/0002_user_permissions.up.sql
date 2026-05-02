CREATE TABLE IF NOT EXISTS user_permissions (
    subject TEXT NOT NULL,
    namespace TEXT NOT NULL,
    resource TEXT NOT NULL,
    verb TEXT NOT NULL,
    granted_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (subject, namespace, resource, verb)
);

CREATE INDEX IF NOT EXISTS idx_user_permissions_subject ON user_permissions(subject);

CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action      TEXT        NOT NULL,
    resource    TEXT        NOT NULL,
    resource_id TEXT,
    ip          TEXT,
    user_agent  TEXT,
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_user_id   ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_action     ON audit_log(action);

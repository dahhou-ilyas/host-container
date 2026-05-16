ALTER TABLE containers
    ADD COLUMN restart_count   INT         NOT NULL DEFAULT 0,
    ADD COLUMN last_restart_at TIMESTAMPTZ,
    ADD COLUMN health_status   TEXT        NOT NULL DEFAULT 'none'
        CHECK (health_status IN ('none','healthy','unhealthy','starting')),
    ADD COLUMN oom_killed      BOOL        NOT NULL DEFAULT false,
    ADD COLUMN circuit_open    BOOL        NOT NULL DEFAULT false;

ALTER TABLE plans
    ADD COLUMN max_auto_restarts INT NOT NULL DEFAULT 3;

UPDATE plans SET max_auto_restarts = 3  WHERE id = 'free';
UPDATE plans SET max_auto_restarts = 10 WHERE id = 'pro';
UPDATE plans SET max_auto_restarts = 30 WHERE id = 'team';

ALTER TABLE templates
    ADD COLUMN healthcheck_test     TEXT[],
    ADD COLUMN healthcheck_interval TEXT,
    ADD COLUMN healthcheck_timeout  TEXT,
    ADD COLUMN healthcheck_retries  INT;

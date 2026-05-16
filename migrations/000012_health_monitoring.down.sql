ALTER TABLE containers
    DROP COLUMN IF EXISTS restart_count,
    DROP COLUMN IF EXISTS last_restart_at,
    DROP COLUMN IF EXISTS health_status,
    DROP COLUMN IF EXISTS oom_killed,
    DROP COLUMN IF EXISTS circuit_open;

ALTER TABLE plans DROP COLUMN IF EXISTS max_auto_restarts;

ALTER TABLE templates
    DROP COLUMN IF EXISTS healthcheck_test,
    DROP COLUMN IF EXISTS healthcheck_interval,
    DROP COLUMN IF EXISTS healthcheck_timeout,
    DROP COLUMN IF EXISTS healthcheck_retries;

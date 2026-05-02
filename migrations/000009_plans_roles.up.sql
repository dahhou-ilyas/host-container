CREATE TABLE plans (
    id                TEXT    PRIMARY KEY,
    name              TEXT    NOT NULL,
    max_containers    INT     NOT NULL DEFAULT 3,
    max_memory_mb     INT     NOT NULL DEFAULT 512,
    max_cpu_nanocores BIGINT  NOT NULL DEFAULT 1000000000,
    auto_stop_minutes INT     NOT NULL DEFAULT 30,
    price_usd_cents   INT     NOT NULL DEFAULT 0
);

INSERT INTO plans VALUES
    ('free', 'Free',  3,  512,  1000000000, 30,  0),
    ('pro',  'Pro',   10, 2048, 2000000000, 120, 900),
    ('team', 'Team',  30, 4096, 4000000000, 480, 2900);

ALTER TABLE users
    ADD COLUMN plan_id TEXT NOT NULL DEFAULT 'free' REFERENCES plans(id),
    ADD COLUMN role    TEXT NOT NULL DEFAULT 'user'
        CHECK (role IN ('user', 'admin'));

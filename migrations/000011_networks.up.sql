CREATE TABLE networks (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name             TEXT        NOT NULL,
    docker_net_id    TEXT        NOT NULL UNIQUE,
    docker_net_name  TEXT        NOT NULL UNIQUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, name)
);

CREATE TABLE container_networks (
    container_id BIGINT NOT NULL REFERENCES containers(id) ON DELETE CASCADE,
    network_id   UUID   NOT NULL REFERENCES networks(id)   ON DELETE CASCADE,
    alias        TEXT   NOT NULL,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (container_id, network_id)
);

CREATE INDEX idx_networks_user_id          ON networks(user_id);
CREATE INDEX idx_container_networks_net_id ON container_networks(network_id);

ALTER TABLE plans ADD COLUMN max_networks INT NOT NULL DEFAULT 1;
UPDATE plans SET max_networks = 1  WHERE id = 'free';
UPDATE plans SET max_networks = 5  WHERE id = 'pro';
UPDATE plans SET max_networks = 20 WHERE id = 'team';

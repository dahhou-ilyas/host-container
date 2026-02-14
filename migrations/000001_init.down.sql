BEGIN;


CREATE TABLE users (
  id        BIGSERIAL PRIMARY KEY,
  name      TEXT,
  email     TEXT UNIQUE,
  password_hash TEXT
);

CREATE TABLE containers (
  id           BIGSERIAL PRIMARY KEY,
  container_id TEXT NOT NULL UNIQUE,
  project_name TEXT NOT NULL,
  folder_path  TEXT NOT NULL,
  port         TEXT,
  status       TEXT NOT NULL,
  user_id      BIGINT NOT NULL,
  CONSTRAINT fk_containers_user
    FOREIGN KEY (user_id) REFERENCES users(id)
    ON DELETE CASCADE
);

CREATE INDEX idx_containers_user_id ON containers(user_id);


COMMIT;

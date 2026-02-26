BEGIN;

CREATE TABLE images (
  id          BIGSERIAL PRIMARY KEY,
  name        TEXT NOT NULL,
  tag         TEXT NOT NULL DEFAULT 'latest',
  created_at  TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_images_name ON images(name);

INSERT INTO images (name, tag) VALUES
  ('nginx',       'latest'),
  ('postgres',    'latest'),
  ('redis',       'latest'),
  ('node',        'latest'),
  ('python',      'latest'),
  ('ubuntu',      'latest'),
  ('alpine',      'latest'),
  ('mysql',       'latest'),
  ('mongo',       'latest'),
  ('rabbitmq',    'latest');

COMMIT;
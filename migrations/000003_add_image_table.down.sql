BEGIN;

DELETE FROM images
WHERE name IN (
  'nginx', 'postgres', 'redis', 'node', 'python',
  'ubuntu', 'alpine', 'mysql', 'mongo', 'rabbitmq'
);

DROP INDEX IF EXISTS idx_images_name;

DROP TABLE IF EXISTS images;

COMMIT;
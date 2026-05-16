DROP TABLE IF EXISTS container_networks;
DROP TABLE IF EXISTS networks;
ALTER TABLE plans DROP COLUMN IF EXISTS max_networks;

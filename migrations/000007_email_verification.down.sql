DROP TABLE IF EXISTS email_tokens;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;

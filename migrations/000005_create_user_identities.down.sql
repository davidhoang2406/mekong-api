ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
DROP TABLE IF EXISTS user_identities;

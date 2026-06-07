CREATE TABLE user_identities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE (provider, provider_id)
);

CREATE INDEX idx_user_identities_user ON user_identities(user_id);

-- Allow social users to have no password
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

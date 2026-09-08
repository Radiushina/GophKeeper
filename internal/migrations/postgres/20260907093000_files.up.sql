CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    byte_size BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL,
    nonce BYTEA NOT NULL,
    meta TEXT NOT NULL DEFAULT '',
    chunk_count INTEGER NOT NULL DEFAULT 0,
    ciphertext_sha256 BYTEA,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS files_user_updated_idx ON files (user_id, updated_at);
CREATE INDEX IF NOT EXISTS files_user_meta_idx ON files (user_id, meta);

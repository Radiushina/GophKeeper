CREATE TABLE IF NOT EXISTS users(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    login VARCHAR(200) NOT NULL UNIQUE,
    password VARCHAR(60) NOT NULL,
    kdf_salt BYTEA NOT NULL,
    kdf_memory INTEGER NOT NULL,
    kdf_iterations INTEGER NOT NULL,
    kdf_parallelism INTEGER NOT NULL,
    kdf_version INTEGER NOT NULL,
    protected_key BYTEA NOT NULL,
    key_hash BYTEA NOT NULL
);

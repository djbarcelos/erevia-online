-- +goose Up
CREATE TABLE accounts (
    id            BIGSERIAL PRIMARY KEY,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    banned        BOOLEAN NOT NULL DEFAULT false
);

-- Liga uma conta externa (Google, Discord, dev) a uma conta do jogo.
CREATE TABLE identities (
    provider     TEXT NOT NULL,
    subject      TEXT NOT NULL,
    account_id   BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);
CREATE INDEX identities_account_idx ON identities(account_id);

-- Só o hash SHA-256 do token fica no banco.
CREATE TABLE sessions (
    token_hash BYTEA PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_account_idx ON sessions(account_id);

CREATE TABLE characters (
    id           BIGSERIAL PRIMARY KEY,
    account_id   BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    class        TEXT NOT NULL CHECK (class IN ('knight', 'barbarian', 'mage', 'rogue')),
    x            REAL NOT NULL DEFAULT 0,
    z            REAL NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX characters_name_lower_idx ON characters (lower(name));
CREATE INDEX characters_account_idx ON characters(account_id);

-- +goose Down
DROP TABLE characters;
DROP TABLE sessions;
DROP TABLE identities;
DROP TABLE accounts;

-- +goose Up
-- Fase 4: PvP (karma), encantamento, clãs e armazém da conta.
ALTER TABLE characters
    ADD COLUMN karma INT NOT NULL DEFAULT 0 CHECK (karma >= 0),
    ADD COLUMN pk_count INT NOT NULL DEFAULT 0 CHECK (pk_count >= 0),
    ADD COLUMN pvp_kills INT NOT NULL DEFAULT 0 CHECK (pvp_kills >= 0),
    ADD COLUMN clan_left_at TIMESTAMPTZ NULL; -- saiu de um clã: espera para entrar em outro

ALTER TABLE character_items
    ADD COLUMN enchant SMALLINT NOT NULL DEFAULT 0 CHECK (enchant BETWEEN 0 AND 10);

CREATE TABLE clans (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    leader_id  BIGINT NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    notice     TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX clans_name_lower_idx ON clans (lower(name));

CREATE TABLE clan_members (
    character_id BIGINT PRIMARY KEY REFERENCES characters (id) ON DELETE CASCADE,
    clan_id      BIGINT NOT NULL REFERENCES clans (id) ON DELETE CASCADE,
    rank         SMALLINT NOT NULL DEFAULT 0 CHECK (rank BETWEEN 0 AND 2), -- 0 membro, 1 oficial, 2 líder
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX clan_members_clan_idx ON clan_members (clan_id);

-- Armazém: compartilhado pelos personagens da conta.
ALTER TABLE accounts
    ADD COLUMN stored_gold BIGINT NOT NULL DEFAULT 0 CHECK (stored_gold >= 0),
    ADD COLUMN storage_seq BIGINT NOT NULL DEFAULT 0;

CREATE TABLE account_items (
    account_id BIGINT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    slot       SMALLINT NOT NULL CHECK (slot >= 0),
    item_id    TEXT NOT NULL,
    count      INT NOT NULL CHECK (count > 0),
    enchant    SMALLINT NOT NULL DEFAULT 0 CHECK (enchant BETWEEN 0 AND 10),
    PRIMARY KEY (account_id, slot)
);

-- +goose Down
DROP TABLE account_items;
ALTER TABLE accounts
    DROP COLUMN storage_seq,
    DROP COLUMN stored_gold;
DROP TABLE clan_members;
DROP TABLE clans;
ALTER TABLE character_items
    DROP COLUMN enchant;
ALTER TABLE characters
    DROP COLUMN clan_left_at,
    DROP COLUMN pvp_kills,
    DROP COLUMN pk_count,
    DROP COLUMN karma;

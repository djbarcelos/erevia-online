-- +goose Up
-- Moderação: quem é GM, silêncio no chat e motivo do banimento (o campo banned já existe).
ALTER TABLE accounts ADD COLUMN gm BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE accounts ADD COLUMN muted_until TIMESTAMPTZ;
ALTER TABLE accounts ADD COLUMN ban_reason TEXT NOT NULL DEFAULT '';

-- Tudo o que um GM faz fica registrado (sem chave estrangeira: o histórico sobrevive à conta).
CREATE TABLE gm_log (
    id            BIGSERIAL PRIMARY KEY,
    at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    gm_account_id BIGINT NOT NULL,
    gm_name       TEXT NOT NULL,
    command       TEXT NOT NULL,
    target        TEXT NOT NULL DEFAULT '',
    detail        TEXT NOT NULL DEFAULT ''
);
CREATE INDEX gm_log_at_idx ON gm_log (at DESC);

-- /bug texto: o relato vem com onde o personagem estava e qual versão rodava.
CREATE TABLE bug_reports (
    id             BIGSERIAL PRIMARY KEY,
    at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    account_id     BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    character_id   BIGINT,
    character_name TEXT NOT NULL,
    level          INT NOT NULL DEFAULT 0,
    x              REAL NOT NULL DEFAULT 0,
    z              REAL NOT NULL DEFAULT 0,
    version        TEXT NOT NULL DEFAULT '',
    user_agent     TEXT NOT NULL DEFAULT '',
    text           TEXT NOT NULL,
    resolved       BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX bug_reports_open_idx ON bug_reports (at DESC) WHERE NOT resolved;

-- +goose Down
DROP TABLE bug_reports;
DROP TABLE gm_log;
ALTER TABLE accounts DROP COLUMN ban_reason;
ALTER TABLE accounts DROP COLUMN muted_until;
ALTER TABLE accounts DROP COLUMN gm;

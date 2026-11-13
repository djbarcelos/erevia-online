-- +goose Up
-- Fase B: atributos com pontos livres, árvore de skills e barra de atalhos no servidor.
ALTER TABLE characters
    ADD COLUMN attr_str    SMALLINT NOT NULL DEFAULT 0 CHECK (attr_str >= 0),
    ADD COLUMN attr_dex    SMALLINT NOT NULL DEFAULT 0 CHECK (attr_dex >= 0),
    ADD COLUMN attr_con    SMALLINT NOT NULL DEFAULT 0 CHECK (attr_con >= 0),
    ADD COLUMN attr_int    SMALLINT NOT NULL DEFAULT 0 CHECK (attr_int >= 0),
    ADD COLUMN attr_wis    SMALLINT NOT NULL DEFAULT 0 CHECK (attr_wis >= 0),
    ADD COLUMN attr_spi    SMALLINT NOT NULL DEFAULT 0 CHECK (attr_spi >= 0),
    -- {"<id da skill>": rank}. Vazio = personagem de antes da árvore (ganha as iniciais ao entrar).
    ADD COLUMN skill_ranks JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- ["skill:5", "item:potion_hp_s", "", ...]. Vazio = barra padrão.
    ADD COLUMN hotbar      JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE characters
    DROP COLUMN attr_str,
    DROP COLUMN attr_dex,
    DROP COLUMN attr_con,
    DROP COLUMN attr_int,
    DROP COLUMN attr_wis,
    DROP COLUMN attr_spi,
    DROP COLUMN skill_ranks,
    DROP COLUMN hotbar;

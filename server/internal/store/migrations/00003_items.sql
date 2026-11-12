-- +goose Up
-- Fase 3: moedas, itens e ordem dos salvamentos.
ALTER TABLE characters
    ADD COLUMN gold      BIGINT NOT NULL DEFAULT 0 CHECK (gold >= 0),
    ADD COLUMN save_seq  BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN kit_given BOOLEAN NOT NULL DEFAULT false; -- kit inicial entregue no primeiro carregamento

-- Bolsa (place 0, slot = espaço) e equipamento (place 1, slot = encaixe).
CREATE TABLE character_items (
    character_id BIGINT   NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    place        SMALLINT NOT NULL CHECK (place IN (0, 1)),
    slot         SMALLINT NOT NULL CHECK (slot >= 0),
    item_id      TEXT     NOT NULL,
    count        INT      NOT NULL CHECK (count > 0),
    PRIMARY KEY (character_id, place, slot)
);
CREATE INDEX character_items_item_idx ON character_items(item_id);

-- +goose Down
DROP TABLE character_items;
ALTER TABLE characters
    DROP COLUMN gold,
    DROP COLUMN save_seq,
    DROP COLUMN kit_given;

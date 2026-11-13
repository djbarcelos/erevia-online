-- +goose Up
-- Fase C: cor do emblema escolhida na criação ('' = padrão da classe, personagens antigos).
ALTER TABLE characters
    ADD COLUMN emblem TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE characters
    DROP COLUMN emblem;

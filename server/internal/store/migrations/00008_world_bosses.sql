-- +goose Up
-- Renascimento dos raid bosses (horas, em relógio de parede): reiniciar o servidor não
-- zera a contagem. Uma linha por chefe (id da def, ex.: 'ozrhal').
CREATE TABLE world_bosses (
    id         TEXT PRIMARY KEY,
    respawn_at TIMESTAMPTZ NOT NULL,
    killed_at  TIMESTAMPTZ,
    top_name   TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE world_bosses;

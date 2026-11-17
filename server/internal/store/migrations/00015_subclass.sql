-- +goose Up
-- Especialização (segunda classe) escolhida no nível 20 pela missão "A Encruzilhada":
-- id do caminho (game.Specs; '' = ainda não escolheu).
ALTER TABLE characters
    ADD COLUMN spec TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE characters
    DROP COLUMN spec;

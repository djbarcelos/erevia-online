-- +goose Up
-- Vitalidade (tunnel#150): XP bônus guardada que dobra a XP de monstro até acabar. Enche fora
-- do jogo (a zona calcula no login a partir de last_seen_at) e parado na cidade. Todo mundo
-- começa em 0 e quem volta já ganha o que passou desde o último salvamento.
ALTER TABLE characters ADD COLUMN vitality BIGINT NOT NULL DEFAULT 0 CHECK (vitality >= 0);

-- +goose Down
ALTER TABLE characters DROP COLUMN vitality;

-- +goose Up
-- Maestria (progressão depois do nível 30): game.MasteryState em JSON (barra, bônus diário,
-- Redistribuir e os pontos de cada nó) e o nível de Maestria (pontos ganhos) numa coluna
-- própria, para o ranking ordenar sem abrir o JSON. Telemetria: kind 'mastery' em
-- game_events (ref = 'point', amount = Maestria nova, extra.gained = pontos de uma vez).
ALTER TABLE characters
    ADD COLUMN mastery JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN mastery_level INT NOT NULL DEFAULT 0;

-- Ranking de nível: no 30, a Maestria desempata antes da XP.
CREATE INDEX characters_rank_mastery_idx ON characters (level DESC, mastery_level DESC, xp DESC, id);

-- +goose Down
DROP INDEX characters_rank_mastery_idx;
ALTER TABLE characters
    DROP COLUMN mastery_level,
    DROP COLUMN mastery;

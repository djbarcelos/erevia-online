-- +goose Up
-- Conquistas do personagem (game.AchievementState em JSON: desbloqueadas e progresso) e o
-- título em uso (id da conquista; '' = nenhum).
ALTER TABLE characters
    ADD COLUMN achievements JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN title TEXT NOT NULL DEFAULT '';

-- Ranking (calculado por um worker a cada poucos minutos): top por nível e por vitórias PvP.
CREATE INDEX characters_rank_level_idx ON characters (level DESC, xp DESC, id);
CREATE INDEX characters_rank_pvp_idx ON characters (pvp_kills DESC, level DESC, xp DESC, id) WHERE pvp_kills > 0;

-- +goose Down
DROP INDEX characters_rank_pvp_idx;
DROP INDEX characters_rank_level_idx;
ALTER TABLE characters
    DROP COLUMN title,
    DROP COLUMN achievements;

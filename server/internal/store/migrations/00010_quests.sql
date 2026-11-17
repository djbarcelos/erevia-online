-- +goose Up
-- Missões do personagem: progresso das ativas e as concluídas (game.QuestState em JSON).
ALTER TABLE characters ADD COLUMN quests JSONB NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE characters DROP COLUMN quests;

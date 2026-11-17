-- +goose Up
-- Bestiário do personagem (game.BestiaryState em JSON): abates e a faixa de níveis derrotados
-- de cada tipo de monstro, e as famílias completas (quando). Salvo junto com o personagem,
-- como as conquistas. Telemetria: kind 'bestiary' em game_events (ref = monstro e amount =
-- marco 1–3, ou ref = 'family:<id>' quando uma família fica completa).
ALTER TABLE characters
    ADD COLUMN bestiary JSONB NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE characters
    DROP COLUMN bestiary;

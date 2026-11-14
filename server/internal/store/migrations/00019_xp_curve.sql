-- +goose Up
-- Curva de XP nova e nível máximo 40 (tunnel#149). A XP fica salva no personagem e o nível
-- sai dela (game.LevelForXP), então mudar a curva mudaria o nível de quem já joga. Esta
-- coluna guarda a versão da curva em que a XP foi salva (game.XPCurve): quem ainda está na
-- 1 é convertido em Go ao carregar o personagem (store.CharacterForAccount chama
-- game.ConvertXP: o mesmo nível e a mesma fração dele) e salvo na 2. Os personagens de
-- hoje ficam na 1; os novos já nascem na 2.
ALTER TABLE characters ADD COLUMN xp_curve SMALLINT NOT NULL DEFAULT 1;
ALTER TABLE characters ALTER COLUMN xp_curve SET DEFAULT 2;

-- +goose Down
ALTER TABLE characters DROP COLUMN xp_curve;

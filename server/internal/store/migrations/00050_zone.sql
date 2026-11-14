-- +goose Up
-- Zonas (vários mapas no mesmo servidor): o mapa onde o personagem está. Os personagens
-- de antes ficam no mapa principal ("starter"). A zona grava o id do mapa a cada
-- salvamento; no portão, o do destino (com a posição de chegada).
ALTER TABLE characters ADD COLUMN zone TEXT NOT NULL DEFAULT 'starter';

-- +goose Down
ALTER TABLE characters DROP COLUMN zone;

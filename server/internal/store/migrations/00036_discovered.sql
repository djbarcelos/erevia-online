-- +goose Up
-- Descoberta de Pedras de Teleporte (game.DiscoveryState em JSON): as pedras que o personagem
-- tocou a pé e a última cidade visitada (destino do Pergaminho de Retorno). Salvo junto com o
-- personagem. Quem já existe ganha as 5 pedras do mapa principal de antes do Brejo; as do
-- Brejo precisam ser descobertas andando. Personagem novo começa vazio (o servidor dá a da cidade).
ALTER TABLE characters
    ADD COLUMN discovered JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE characters
   SET discovered = '{"stones": ["cidade", "arredores", "ossos", "ermos", "vale"]}'::jsonb;

-- +goose Down
ALTER TABLE characters
    DROP COLUMN discovered;

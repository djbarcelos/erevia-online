-- +goose Up
-- Contratos do nível máximo e reputação com as facções (tunnel#157).
--   contracts:  game.ContractState em JSON: o dia e a semana (Brasília) e os contratos
--               aceitos ({"day": 20770, "week": 2966, "taken": {"hunt_vale_golems": {"n": 40}}}).
--               Os contratos do dia não ficam salvos: o sorteio é determinístico por
--               personagem e dia (server/internal/game/contracts.go).
--   reputation: game.ReputationState em JSON: os pontos por facção ({"points": {"bastion": 820}}).
-- Telemetria em game_events: kind 'contract_accept' (ref = contrato) e 'contract_done'
-- (ref = contrato, amount = moedas, extra = vaga, facção, reputação ganha, XP, dia e semana);
-- as moedas também vão em kind 'gold' (ref 'contract' e 'rep_shop') e os itens da loja em
-- 'item_in' (extra.how = 'rep_shop').
ALTER TABLE characters
    ADD COLUMN contracts JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN reputation JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Painel: quantos aceitaram e concluíram cada contrato por dia (Brasília).
CREATE VIEW v_contracts_daily AS
SELECT (at AT TIME ZONE 'America/Sao_Paulo')::date AS dia,
       ref AS contrato,
       count(*) FILTER (WHERE kind = 'contract_accept') AS aceitos,
       count(*) FILTER (WHERE kind = 'contract_done') AS concluidos,
       coalesce(sum(amount) FILTER (WHERE kind = 'contract_done'), 0) AS moedas
FROM game_events
WHERE kind IN ('contract_accept', 'contract_done')
GROUP BY 1, 2;

-- +goose Down
DROP VIEW v_contracts_daily;
ALTER TABLE characters
    DROP COLUMN reputation,
    DROP COLUMN contracts;

-- +goose Up
-- Apagar a conta anonimiza a telemetria dela (game_events.account_id). Sem índice, isso
-- varreria a tabela inteira, que só cresce.
CREATE INDEX game_events_account_idx ON game_events (account_id) WHERE account_id IS NOT NULL;

-- +goose Down
DROP INDEX game_events_account_idx;

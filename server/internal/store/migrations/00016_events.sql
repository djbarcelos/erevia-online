-- +goose Up
-- Eventos ligados por GM com prazo (XP em dobro: /evento xp2 horas|off), para valer
-- depois de um reinício do servidor. Uma linha por evento ('xp2', 'xp2_off'). A invasão
-- da cidade não fica aqui: dura minutos e um reinício simplesmente a encerra.
CREATE TABLE server_events (
    id         TEXT PRIMARY KEY,
    until      TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE server_events;

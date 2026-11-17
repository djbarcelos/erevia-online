-- +goose Up
-- Alertas no Discord (server/internal/alerts): o que já foi avisado e quando, para o limite
-- e o "já visto" valerem depois de um reinício. Uma linha por (tipo, chave):
--   seen       client:<hash> | server:<hash>  erro já visto (avisado, ou anterior aos alertas)
--   spike      client:<hash> | server:<hash> | db:<operação>  último aviso de pico
--   invariant  <regra>@<AAAA-MM-DD>           a primeira quebra da regra no dia (Brasília)
--   slow_tick  zone                           último aviso de tick lento
--   bug        <account_id>                   último aviso de /bug da conta; pos = último id avisado
--   boot       <id da subida>                 cada subida do servidor: info = versão e se
--                                              desligou normalmente; pos = último sinal (unix)
--   boot_alert <versão>                       último aviso de "servidor no ar" da versão
--   cursor     bootstrap                      primeira vez dos alertas; pos = maior id de
--                                              bug_reports naquela hora (o que veio antes não é avisado)
-- Sem chave estrangeira: o estado dos avisos não pode travar nem depender das outras tabelas.
CREATE TABLE alert_state (
    kind       TEXT NOT NULL,
    key        TEXT NOT NULL,
    sent_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    pos        BIGINT NOT NULL DEFAULT 0,
    -- Avisos iguais segurados pelo limite desde o último que saiu.
    suppressed INT NOT NULL DEFAULT 0,
    info       JSONB,
    PRIMARY KEY (kind, key)
);
-- A limpeza (avisos de mais de 30 dias) e a subida anterior procuram por tipo e data.
CREATE INDEX alert_state_kind_sent_idx ON alert_state (kind, sent_at);

-- +goose Down
DROP TABLE alert_state;

-- +goose Up
-- Portal de administração (ações): o histórico de moderação de uma conta procura no gm_log
-- pelo nome dos personagens dela (sem diferenciar maiúsculas).
CREATE INDEX gm_log_target_idx ON gm_log (lower(target), at DESC);

-- Anúncios agendados pelo portal: a zona guarda a fila em memória e solta na hora; aqui
-- eles sobrevivem a um reinício (sem chave estrangeira, como o gm_log).
CREATE TABLE scheduled_announcements (
    id              BIGSERIAL PRIMARY KEY,
    at              TIMESTAMPTZ NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    text            TEXT NOT NULL,
    created_by      BIGINT NOT NULL,
    created_by_name TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    cancelled_at    TIMESTAMPTZ
);
CREATE INDEX scheduled_announcements_pending_idx ON scheduled_announcements (at) WHERE cancelled_at IS NULL;

-- +goose Down
DROP TABLE scheduled_announcements;
DROP INDEX gm_log_target_idx;

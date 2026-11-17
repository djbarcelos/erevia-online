-- +goose Up
-- /bug completo: o relato feito pelo jogo (chat ou o botão "Relatar problema") traz o
-- contexto do cliente (erros recentes, FPS, ping, região, hora do jogo, últimas mensagens
-- do servidor; formato no README, "Moderação") e um print pequeno do canvas.
--
-- O contexto fica na própria linha (JSON, até 32 KB). O print fica numa tabela à parte
-- para as listagens de bug_reports não arrastarem o JPEG; os dois somem junto com a conta
-- (bug_reports.account_id já é ON DELETE CASCADE, e o print cai em cascata do relato).
ALTER TABLE bug_reports ADD COLUMN context JSONB;

CREATE TABLE bug_report_shots (
    report_id BIGINT PRIMARY KEY REFERENCES bug_reports (id) ON DELETE CASCADE,
    at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    width     INT NOT NULL,
    height    INT NOT NULL,
    jpeg      BYTEA NOT NULL
);

-- +goose Down
DROP TABLE bug_report_shots;
ALTER TABLE bug_reports DROP COLUMN context;

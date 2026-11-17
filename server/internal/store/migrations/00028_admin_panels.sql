-- +goose Up
-- Portal de administração, parte de leitura (Relatos, Métricas e Economia).

-- Relato de /bug resolvido: quem resolveu (a conta GM e o nome, sem chave estrangeira,
-- como no gm_log), quando e a nota. Reabrir limpa os quatro.
ALTER TABLE bug_reports
    ADD COLUMN resolved_at      TIMESTAMPTZ,
    ADD COLUMN resolved_by      BIGINT,
    ADD COLUMN resolved_by_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN resolved_note    TEXT NOT NULL DEFAULT '';
-- A lista do portal pagina por id (do mais novo ao mais velho), com e sem os resolvidos.
CREATE INDEX bug_reports_resolved_id_idx ON bug_reports (resolved, id DESC);

-- Painéis de métricas e economia (store/admin_metrics.go, admin_economy.go). Medido com
-- uma semana de eventos sintéticos (1 milhão de linhas, store/admin_perf_test.go): sem
-- estes índices, cada consulta lia a tabela quase toda (os tipos se misturam em toda
-- página) e levava de 0,5 a 1,5 s; com eles, viram index-only scans.
--
-- (kind, at) com as colunas que as contas leem: substitui game_events_kind_at_idx (mesma
-- chave), então são 2 índices a mais na gravação, não 3.
CREATE INDEX game_events_kind_at_cover_idx ON game_events (kind, at)
    INCLUDE (character_id, level, ref, amount, x, z);
DROP INDEX game_events_kind_at_idx;
-- Itens e FPS também leem extra (how; device e fps_low), que é pequeno nesses tipos.
-- Os erros do servidor ficam de fora: o extra deles leva a stack.
CREATE INDEX game_events_items_idx ON game_events (at) INCLUDE (kind, ref, amount, extra)
    WHERE kind IN ('item_in', 'item_out');
CREATE INDEX game_events_fps_idx ON game_events (at) INCLUDE (x, z, amount, extra)
    WHERE kind = 'fps';
-- Diz ao planejador que região × aparelho são poucos grupos (sem isto ele estima um grupo
-- por linha e ordena tudo em disco em vez de agregar num hash).
CREATE STATISTICS game_events_fps_stats (ndistinct) ON (erevia_region(x, z)), (extra->>'device') FROM game_events;
-- Index-only scan só pula a tabela nas páginas marcadas como visíveis pelo vacuum. A
-- tabela só recebe inserções: vacuum a cada 2% de linhas novas (o padrão é 20%), para a
-- semana mais recente não ficar sem marca.
ALTER TABLE game_events SET (autovacuum_vacuum_insert_scale_factor = 0.02);

-- +goose Down
ALTER TABLE game_events RESET (autovacuum_vacuum_insert_scale_factor);
DROP STATISTICS game_events_fps_stats;
DROP INDEX game_events_fps_idx;
DROP INDEX game_events_items_idx;
CREATE INDEX game_events_kind_at_idx ON game_events (kind, at);
DROP INDEX game_events_kind_at_cover_idx;
DROP INDEX bug_reports_resolved_id_idx;
ALTER TABLE bug_reports
    DROP COLUMN resolved_note,
    DROP COLUMN resolved_by_name,
    DROP COLUMN resolved_by,
    DROP COLUMN resolved_at;

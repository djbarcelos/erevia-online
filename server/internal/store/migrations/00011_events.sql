-- +goose Up
-- Telemetria: um evento por linha, gravado em lote pela zona (ver zone/telemetry.go).
--   gold      amount = moedas (+ entrou / − saiu), ref = de onde (loot, sell, quest, gm, buy, craft…)
--   item_in   ref = item, amount = quantidade, extra.how = loot, buy, craft, quest, gm
--   item_out  ref = item, amount = quantidade, extra.how = sell, destroy, enchant_break, craft
--   kill      ref = monstro, amount = nível do monstro
--   death     ref = quem matou ("monster:<id>", "player" ou "")
--   level     amount = nível novo
--   session   ref = start | end, amount = segundos jogados (no end)
--   fps       amount = FPS médio, extra = fps_low, device, seconds
CREATE TABLE game_events (
    id           BIGSERIAL PRIMARY KEY,
    at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    kind         TEXT NOT NULL,
    character_id BIGINT,
    account_id   BIGINT,
    level        INT NOT NULL DEFAULT 0,
    x            REAL NOT NULL DEFAULT 0,
    z            REAL NOT NULL DEFAULT 0,
    ref          TEXT NOT NULL DEFAULT '',
    amount       BIGINT NOT NULL DEFAULT 0,
    extra        JSONB
);
CREATE INDEX game_events_kind_at_idx ON game_events (kind, at);
CREATE INDEX game_events_character_idx ON game_events (character_id, at);

-- Região aproximada pela posição (mesmos anéis do mapa; o Vale fica ao sul).
CREATE FUNCTION erevia_region(x REAL, z REAL) RETURNS TEXT LANGUAGE SQL IMMUTABLE AS $$
    SELECT CASE
        WHEN z < -200 THEN 'Vale das Pedras Vivas'
        WHEN sqrt(x*x + z*z) < 40 THEN 'Cidade'
        WHEN sqrt(x*x + z*z) < 78 THEN 'Arredores'
        WHEN sqrt(x*x + z*z) < 112 THEN 'Campos dos Ossos'
        ELSE 'Ermos Partidos'
    END
$$;

-- "Painel": abrir estas views na aba Data do Postgres da Railway (ou com psql).

-- Ouro criado e retirado por dia, por origem.
CREATE VIEW v_gold_daily AS
SELECT date_trunc('day', at)::date AS dia, ref AS origem,
       sum(amount) FILTER (WHERE amount > 0) AS entrou,
       -sum(amount) FILTER (WHERE amount < 0) AS saiu
FROM game_events WHERE kind = 'gold'
GROUP BY 1, 2 ORDER BY 1 DESC, 2;

-- Tempo (relógio) para chegar a cada nível, pela diferença entre subidas do mesmo personagem.
CREATE VIEW v_level_times AS
WITH ups AS (
    SELECT character_id, amount AS nivel, at,
           lag(at) OVER (PARTITION BY character_id ORDER BY at) AS antes
    FROM game_events WHERE kind = 'level'
)
SELECT nivel, count(*) AS personagens,
       round(avg(extract(epoch FROM at - antes)) / 60) AS minutos_medios
FROM ups WHERE antes IS NOT NULL
GROUP BY nivel ORDER BY nivel;

-- Mortes por região e por quem matou.
CREATE VIEW v_deaths_by_region AS
SELECT erevia_region(x, z) AS regiao, ref AS causa, count(*) AS mortes, round(avg(level), 1) AS nivel_medio
FROM game_events WHERE kind = 'death'
GROUP BY 1, 2 ORDER BY 3 DESC;

-- Último nível de quem não joga há mais de 7 dias (onde as pessoas desistem).
CREATE VIEW v_quit_level AS
SELECT c.level AS nivel, count(*) AS personagens
FROM characters c
WHERE c.last_seen_at < now() - interval '7 days'
GROUP BY 1 ORDER BY 1;

-- FPS médio por região e tipo de aparelho (amostras de 60 s dos clientes).
CREATE VIEW v_fps AS
SELECT erevia_region(x, z) AS regiao, extra->>'device' AS aparelho,
       count(*) AS amostras, round(avg(amount), 1) AS fps_medio,
       round(avg((extra->>'fps_low')::numeric), 1) AS fps_baixo_medio
FROM game_events WHERE kind = 'fps'
GROUP BY 1, 2 ORDER BY 1, 2;

-- Itens que entraram e saíram do jogo por dia (para caçar duplicação).
CREATE VIEW v_items_daily AS
SELECT date_trunc('day', at)::date AS dia, ref AS item, extra->>'how' AS como,
       sum(amount) FILTER (WHERE kind = 'item_in') AS entrou,
       sum(amount) FILTER (WHERE kind = 'item_out') AS saiu
FROM game_events WHERE kind IN ('item_in', 'item_out')
GROUP BY 1, 2, 3 ORDER BY 1 DESC, 2;

-- +goose Down
DROP VIEW v_items_daily;
DROP VIEW v_fps;
DROP VIEW v_quit_level;
DROP VIEW v_deaths_by_region;
DROP VIEW v_level_times;
DROP VIEW v_gold_daily;
DROP FUNCTION erevia_region(REAL, REAL);
DROP TABLE game_events;

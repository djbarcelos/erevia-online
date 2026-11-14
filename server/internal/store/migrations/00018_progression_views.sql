-- +goose Up
-- Progressão real (tunnel#147, docs/progressao.md): quanto tempo de jogo, quantos abates e
-- de onde vem a XP em cada nível. Só views sobre game_events (00011) e characters; nada
-- muda nas tabelas. Evento novo que elas usam (zone/xptally.go):
--   xp   ref = fonte (monster, party, night_bonus, event_bonus, quest, invasion,
--        death_loss), amount = XP (+ ganhou / − perdeu), level = nível em que a XP veio.
--        Somada na zona por personagem e gravada a cada ~60 s, ao subir e ao sair.
--
-- "Tempo de jogo" é a soma das sessões (session start → end), não o relógio: quem joga uma
-- hora por dia não pode parecer mais lento do que quem joga oito.

-- Faixa de nível dos painéis ('01-09' para ordenar antes de '10-19').
CREATE FUNCTION erevia_band(level INT) RETURNS TEXT LANGUAGE SQL IMMUTABLE AS $$
    SELECT CASE WHEN level < 10 THEN '01-09' WHEN level < 20 THEN '10-19' WHEN level < 30 THEN '20-29' ELSE '30' END
$$;

-- Sessões: cada start com o end seguinte do mesmo personagem. Sem end (servidor caiu antes
-- de gravar), a sessão vai até o último evento do personagem antes da próxima sessão (a XP
-- sai a cada minuto, então erra por no máximo ~1 min de quem estava ganhando XP).
CREATE VIEW v_sessions AS
WITH s AS (
    SELECT character_id, at, ref, level,
           lead(at)  OVER w AS next_at,
           lead(ref) OVER w AS next_ref
    FROM game_events
    WHERE kind = 'session' AND character_id IS NOT NULL
    WINDOW w AS (PARTITION BY character_id ORDER BY at, id)
)
SELECT s.character_id, s.level AS nivel_inicio, s.at AS inicio,
       CASE WHEN s.next_ref = 'end' THEN s.next_at
            ELSE coalesce((SELECT max(e.at) FROM game_events e
                           WHERE e.character_id = s.character_id AND e.at >= s.at
                             AND (s.next_at IS NULL OR e.at < s.next_at)), s.at)
       END AS fim,
       coalesce(s.next_ref = 'end', false) AS fechada
FROM s WHERE s.ref = 'start';

-- Trechos: o tempo que cada personagem passou em cada nível. Um trecho começa numa subida
-- (evento level) ou, no primeiro nível visto do personagem, na primeira sessão, e acaba na
-- subida seguinte (fim NULL = ainda está nele). inicio_exato: o trecho começou de fato no
-- começo do nível (subida, ou personagem novo no nível 1); quem já existia antes da
-- telemetria entra nas horas, mas não no tempo por nível.
CREATE VIEW v_level_segments AS
WITH marcos AS (
    SELECT character_id, amount::int AS nivel, at, id, true AS subida
    FROM game_events WHERE kind = 'level' AND character_id IS NOT NULL
    UNION ALL
    SELECT * FROM (
        SELECT DISTINCT ON (character_id) character_id, level, at, id, false
        FROM game_events WHERE kind = 'session' AND ref = 'start' AND character_id IS NOT NULL
        ORDER BY character_id, at, id
    ) primeira
),
trechos AS (
    SELECT character_id, nivel, at AS inicio, subida,
           lead(at)    OVER w AS fim,
           lead(nivel) OVER w AS prox_nivel
    FROM marcos
    WINDOW w AS (PARTITION BY character_id ORDER BY at, id)
)
SELECT t.character_id, t.nivel, t.prox_nivel, t.inicio, t.fim,
       (t.subida OR t.nivel = 1) AS inicio_exato,
       coalesce(sum(extract(epoch FROM least(s.fim, coalesce(t.fim, 'infinity')) - greatest(s.inicio, t.inicio)))
                FILTER (WHERE s.character_id IS NOT NULL), 0)::float8 AS segundos_jogados
FROM trechos t
LEFT JOIN v_sessions s
       ON s.character_id = t.character_id AND s.fim > t.inicio AND (t.fim IS NULL OR s.inicio < t.fim)
GROUP BY t.character_id, t.nivel, t.prox_nivel, t.inicio, t.fim, t.subida;

-- Tempo de jogo para passar de nivel a nivel+1, por classe e no total (classe 'todas').
-- Só trechos completos, que começaram no começo do nível e terminaram no nível seguinte
-- (subir dois níveis de uma vez não entra). Mediana e p75 contam mais do que a média:
-- um personagem esquecido aberto puxa a média para cima.
CREATE VIEW v_level_playtime AS
SELECT t.nivel,
       CASE WHEN grouping(c.class) = 1 THEN 'todas' ELSE coalesce(c.class, '?') END AS classe,
       count(*) AS personagens,
       round((avg(t.segundos_jogados) / 60)::numeric, 1) AS minutos_media,
       round((percentile_cont(0.5) WITHIN GROUP (ORDER BY t.segundos_jogados) / 60)::numeric, 1) AS minutos_mediana,
       round((percentile_cont(0.75) WITHIN GROUP (ORDER BY t.segundos_jogados) / 60)::numeric, 1) AS minutos_p75,
       round((percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM t.fim - t.inicio)) / 3600)::numeric, 1) AS horas_relogio_mediana
FROM v_level_segments t
LEFT JOIN characters c ON c.id = t.character_id
WHERE t.inicio_exato AND t.prox_nivel = t.nivel + 1
GROUP BY GROUPING SETS ((t.nivel), (t.nivel, c.class))
ORDER BY t.nivel, grouping(c.class) DESC, classe;

-- De onde vem a XP em cada faixa (pct: parte da XP ganha na faixa; death_loss sai negativa).
CREATE VIEW v_xp_sources AS
SELECT erevia_band(level) AS faixa, ref AS fonte,
       sum(amount) AS xp,
       count(DISTINCT character_id) AS personagens,
       round(100.0 * sum(amount) / nullif(sum(greatest(sum(amount), 0)) OVER (PARTITION BY erevia_band(level)), 0), 1) AS pct
FROM game_events
WHERE kind = 'xp'
GROUP BY 1, 2
ORDER BY 1, 3 DESC;

-- Abates e mortes por nível do jogador (abates_por_personagem compara com a tabela teórica).
CREATE VIEW v_level_kills_deaths AS
SELECT level AS nivel,
       count(*) FILTER (WHERE kind = 'kill') AS abates,
       count(*) FILTER (WHERE kind = 'death') AS mortes,
       count(DISTINCT character_id) FILTER (WHERE kind IN ('kill', 'death')) AS personagens,
       round(count(*) FILTER (WHERE kind = 'kill')::numeric
             / nullif(count(DISTINCT character_id) FILTER (WHERE kind = 'kill'), 0), 1) AS abates_por_personagem,
       round(count(*) FILTER (WHERE kind = 'kill')::numeric
             / nullif(count(*) FILTER (WHERE kind = 'death'), 0), 1) AS abates_por_morte,
       coalesce(-sum(amount) FILTER (WHERE kind = 'xp' AND ref = 'death_loss'), 0) AS xp_perdida_em_mortes
FROM game_events
WHERE kind IN ('kill', 'death', 'xp') AND level > 0
GROUP BY level
ORDER BY level;

-- Ouro ganho e gasto por hora de jogo, por faixa. O ouro criado por GM (/ouro) não conta.
CREATE VIEW v_gold_per_hour AS
WITH horas AS (
    SELECT erevia_band(nivel) AS faixa, sum(segundos_jogados) / 3600 AS horas,
           count(DISTINCT character_id) AS personagens
    FROM v_level_segments GROUP BY 1
), ouro AS (
    SELECT erevia_band(level) AS faixa,
           coalesce(sum(amount) FILTER (WHERE amount > 0), 0) AS ganho,
           coalesce(-sum(amount) FILTER (WHERE amount < 0), 0) AS gasto
    FROM game_events WHERE kind = 'gold' AND ref <> 'gm' AND level > 0
    GROUP BY 1
)
SELECT faixa, round(coalesce(h.horas, 0)::numeric, 1) AS horas_jogadas, coalesce(h.personagens, 0) AS personagens,
       coalesce(o.ganho, 0) AS ouro_ganho, coalesce(o.gasto, 0) AS ouro_gasto,
       round(o.ganho / nullif(h.horas, 0)) AS ganho_por_hora,
       round(o.gasto / nullif(h.horas, 0)) AS gasto_por_hora,
       round((coalesce(o.ganho, 0) - coalesce(o.gasto, 0)) / nullif(h.horas, 0)) AS saldo_por_hora
FROM horas h FULL JOIN ouro o USING (faixa)
ORDER BY faixa;

-- Funil: quantos personagens chegaram a cada nível, quantos estão nele e quantos pararam
-- nele (sem entrar há mais de 7 dias). Vem de characters, então conta também quem subiu
-- antes da telemetria. pct_do_anterior: dos que chegaram ao nível anterior, quantos chegaram
-- a este.
CREATE VIEW v_level_reached AS
WITH niveis AS (
    SELECT generate_series(1, greatest(30, (SELECT coalesce(max(level), 1) FROM characters))) AS nivel
)
SELECT n.nivel,
       count(c.id) AS chegaram,
       count(c.id) FILTER (WHERE c.level = n.nivel) AS estao_nele,
       count(c.id) FILTER (WHERE c.level = n.nivel AND coalesce(c.last_seen_at, c.created_at) < now() - interval '7 days') AS pararam_nele,
       round(100.0 * count(c.id) / nullif(lag(count(c.id)) OVER (ORDER BY n.nivel), 0), 1) AS pct_do_anterior
FROM niveis n
LEFT JOIN characters c ON c.level >= n.nivel
GROUP BY n.nivel
ORDER BY n.nivel;

-- +goose Down
DROP VIEW v_level_reached;
DROP VIEW v_gold_per_hour;
DROP VIEW v_level_kills_deaths;
DROP VIEW v_xp_sources;
DROP VIEW v_level_playtime;
DROP VIEW v_level_segments;
DROP VIEW v_sessions;
DROP FUNCTION erevia_band(INT);

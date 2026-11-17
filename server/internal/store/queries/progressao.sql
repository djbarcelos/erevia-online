-- Consultas prontas da progressão (tunnel#147, docs/progressao.md).
-- Cole uma de cada vez na aba Data do Postgres da Railway (ou rode com psql).
-- As views vêm da migração 00018; os eventos, de game_events (00011).
-- Também aparecem no portal de administração, em /admin#progressao.

-- 1. Tempo de jogo por nível (minutos de sessão para passar de nível a nível+1).
SELECT * FROM v_level_playtime WHERE classe = 'todas' ORDER BY nivel;

-- 1b. O mesmo, por classe.
SELECT * FROM v_level_playtime WHERE classe <> 'todas' ORDER BY nivel, classe;

-- 2. Horas de jogo por faixa, somando as medianas de cada nível (compare com o tempo-alvo:
--    1→10 em ~3 h, 10→20 em ~15 h, 20→30 em ~40 h).
SELECT CASE WHEN nivel < 10 THEN '1 → 10' WHEN nivel < 20 THEN '10 → 20' ELSE '20 → 30' END AS faixa,
       count(*) AS niveis_com_dados,
       round(sum(minutos_mediana) / 60, 1) AS horas_mediana,
       round(sum(minutos_p75) / 60, 1) AS horas_p75
FROM v_level_playtime WHERE classe = 'todas'
GROUP BY 1 ORDER BY min(nivel);

-- 3. De onde vem a XP em cada faixa (quanto é missão, grupo, noite, XP em dobro; quanto se
--    perde morrendo).
SELECT * FROM v_xp_sources;

-- 4. XP por hora de jogo em cada faixa (todas as fontes, já com as perdas).
SELECT x.faixa, x.xp, g.horas_jogadas, round(x.xp / nullif(g.horas_jogadas, 0)) AS xp_por_hora
FROM (SELECT faixa, sum(xp) AS xp FROM v_xp_sources GROUP BY faixa) x
JOIN v_gold_per_hour g USING (faixa)
ORDER BY x.faixa;

-- 5. Abates e mortes por nível (abates_por_personagem: compare com a coluna "Abates" da
--    tabela teórica do docs/progressao.md).
SELECT * FROM v_level_kills_deaths;

-- 6. Ouro ganho e gasto por hora de jogo, por faixa.
SELECT * FROM v_gold_per_hour;

-- 7. Funil: quantos chegaram a cada nível e onde pararam.
SELECT * FROM v_level_reached;

-- 8. Um personagem específico: cada nível, quando subiu, minutos de jogo e de relógio.
SELECT s.nivel, s.inicio, s.fim, round((s.segundos_jogados / 60)::numeric, 1) AS minutos_jogados,
       round((extract(epoch FROM s.fim - s.inicio) / 60)::numeric) AS minutos_relogio, s.inicio_exato
FROM v_level_segments s JOIN characters c ON c.id = s.character_id
WHERE lower(c.name) = lower('NomeDoPersonagem')
ORDER BY s.inicio;

-- 9. Quanto da XP veio do XP em dobro nos últimos 7 dias (o fim de semana pesa quanto?).
SELECT ref AS fonte, sum(amount) AS xp, round(100.0 * sum(amount) / sum(sum(amount)) OVER (), 1) AS pct
FROM game_events
WHERE kind = 'xp' AND amount > 0 AND at > now() - interval '7 days'
GROUP BY ref ORDER BY xp DESC;

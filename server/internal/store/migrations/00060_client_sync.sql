-- +goose Up
-- Dessincronia cliente × servidor (tunnel#122): o cliente manda, a cada ~60 s (junto com o
-- FPS), quantas vezes corrigiu a posição prevista do próprio personagem, quantas vezes o
-- servidor retomou o movimento, quantas entidades sumiram e voltaram em menos de 1 s e o
-- ping. Evento game_events.kind = 'client_sync' (amount = segundos da amostra).
--
-- Correções por minuto, por região e faixa de ping (p95 da amostra).
CREATE VIEW v_client_sync AS
SELECT erevia_region(x, z) AS regiao,
       CASE WHEN (extra->>'ping_p95')::numeric < 100 THEN '< 100 ms'
            WHEN (extra->>'ping_p95')::numeric < 200 THEN '100–199 ms'
            WHEN (extra->>'ping_p95')::numeric < 400 THEN '200–399 ms'
            ELSE '400+ ms' END AS ping,
       count(*) AS amostras,
       round(sum((extra->>'corrections')::numeric) * 60 / NULLIF(sum(amount), 0), 2) AS correcoes_min,
       round(sum((extra->>'corrections_big')::numeric) * 60 / NULLIF(sum(amount), 0), 2) AS correcoes_2m_min,
       round(sum((extra->>'resyncs')::numeric) * 60 / NULLIF(sum(amount), 0), 2) AS retomadas_min,
       round(sum((extra->>'flickers')::numeric) * 60 / NULLIF(sum(amount), 0), 2) AS piscadas_min,
       round(avg((extra->>'ping_avg')::numeric), 0) AS ping_medio
FROM game_events WHERE kind = 'client_sync'
GROUP BY 1, 2 ORDER BY 1, 2;

-- +goose Down
DROP VIEW v_client_sync;

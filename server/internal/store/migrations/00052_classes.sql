-- +goose Up
-- Subclasse (tunnel#156): as classes do personagem (game.ClassSet). characters.class
-- continua a principal (a do corpo); as colunas de progressão (level, xp, attr_*,
-- skill_ranks, hotbar, spec) são as da classe ativa, e cada classe guarda a sua aqui:
-- {"main", "active", "list": [{"class", "level", "xp", "attrs", "ranks", "hotbar", "spec"}],
-- "switched_at"}. Quem já existe fica com a classe dele como principal e ativa, e o
-- progresso de hoje vira o dela.
ALTER TABLE characters
    ADD COLUMN classes JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE characters SET classes = jsonb_build_object(
    'main', class,
    'active', class,
    'list', jsonb_build_array(jsonb_build_object(
        'class', class,
        'level', level,
        'xp', xp,
        'attrs', jsonb_build_object('Str', attr_str, 'Dex', attr_dex, 'Con', attr_con,
                                    'Int', attr_int, 'Wis', attr_wis, 'Spi', attr_spi),
        'ranks', skill_ranks,
        'hotbar', hotbar,
        'spec', spec)));

-- +goose Down
-- Quem estava com uma subclasse ativa volta para a principal: a progressão dela sai da
-- lista para as colunas antes de a coluna sumir (a subclasse se perde).
UPDATE characters c SET
    level = (m.e->>'level')::int,
    xp = (m.e->>'xp')::bigint,
    attr_str = coalesce((m.e->'attrs'->>'Str')::smallint, 0),
    attr_dex = coalesce((m.e->'attrs'->>'Dex')::smallint, 0),
    attr_con = coalesce((m.e->'attrs'->>'Con')::smallint, 0),
    attr_int = coalesce((m.e->'attrs'->>'Int')::smallint, 0),
    attr_wis = coalesce((m.e->'attrs'->>'Wis')::smallint, 0),
    attr_spi = coalesce((m.e->'attrs'->>'Spi')::smallint, 0),
    skill_ranks = coalesce(m.e->'ranks', '{}'::jsonb),
    hotbar = coalesce(m.e->'hotbar', '[]'::jsonb),
    spec = coalesce(m.e->>'spec', '')
FROM (
    SELECT ch.id, e
    FROM characters ch, jsonb_array_elements(ch.classes->'list') e
    WHERE jsonb_typeof(ch.classes->'list') = 'array'
      AND coalesce(ch.classes->>'active', ch.class) <> ch.class
      AND e->>'class' = ch.class
) m
WHERE c.id = m.id;

ALTER TABLE characters
    DROP COLUMN classes;

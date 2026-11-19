-- +goose Up
-- Arena PvP ranqueada (tunnel#81): a pontuação de cada personagem por temporada (o mês,
-- 202611) e modo (1 = 1x1, 3 = 3x3). Quem nunca lutou na temporada não tem linha (vale
-- game.ArenaStartRating). Apagar o personagem apaga a pontuação.
CREATE TABLE arena_ratings (
    season       integer  NOT NULL,
    mode         smallint NOT NULL CHECK (mode IN (1, 3)),
    character_id bigint   NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    rating       integer  NOT NULL,
    wins         integer  NOT NULL DEFAULT 0,
    losses       integer  NOT NULL DEFAULT 0,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (season, mode, character_id)
);

-- O ranking da temporada: maior pontuação primeiro.
CREATE INDEX arena_ratings_rank_idx ON arena_ratings (season, mode, rating DESC, character_id);

-- +goose Down
DROP TABLE arena_ratings;

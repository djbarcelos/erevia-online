-- +goose Up
-- Visual do corpo (criação e barbeiro). Os padrões são o visual original da classe.
ALTER TABLE characters
    ADD COLUMN skin     SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN hair     SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN outfit   SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN head     TEXT     NOT NULL DEFAULT '',
    ADD COLUMN hide_hat BOOLEAN  NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE characters
    DROP COLUMN skin,
    DROP COLUMN hair,
    DROP COLUMN outfit,
    DROP COLUMN head,
    DROP COLUMN hide_hat;

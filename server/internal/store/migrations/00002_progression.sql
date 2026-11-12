-- +goose Up
-- Progressão: nível, XP total e vida/mana atuais (NULL = cheio).
ALTER TABLE characters
    ADD COLUMN level INT NOT NULL DEFAULT 1 CHECK (level BETWEEN 1 AND 100),
    ADD COLUMN xp    BIGINT NOT NULL DEFAULT 0 CHECK (xp >= 0),
    ADD COLUMN hp    INT,
    ADD COLUMN mp    INT;

-- +goose Down
ALTER TABLE characters
    DROP COLUMN level,
    DROP COLUMN xp,
    DROP COLUMN hp,
    DROP COLUMN mp;

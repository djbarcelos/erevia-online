-- +goose Up
-- Amigos (de mão única, como "seguir") e correio entre personagens (só texto).
CREATE TABLE friends (
    character_id BIGINT NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    friend_id    BIGINT NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, friend_id),
    CHECK (character_id <> friend_id)
);
CREATE INDEX friends_friend_idx ON friends (friend_id);

-- Apagar uma carta só a esconde (deleted_at): o limite de cartas por hora do remetente
-- continua contando. Cartas apagadas há mais de uma hora são removidas de vez.
CREATE TABLE mail (
    id         BIGSERIAL PRIMARY KEY,
    to_id      BIGINT NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    from_id    BIGINT REFERENCES characters (id) ON DELETE SET NULL,
    from_name  TEXT NOT NULL,
    subject    TEXT NOT NULL DEFAULT '',
    body       TEXT NOT NULL,
    sent_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    read       BOOLEAN NOT NULL DEFAULT false,
    deleted_at TIMESTAMPTZ
);
CREATE INDEX mail_inbox_idx ON mail (to_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX mail_sender_idx ON mail (from_id, sent_at);

-- +goose Down
DROP TABLE mail;
DROP TABLE friends;

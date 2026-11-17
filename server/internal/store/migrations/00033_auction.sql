-- +goose Up
-- Casa de leilões (tunnel#78). O item anunciado fica aqui em custódia até ser vendido,
-- cancelado ou vencer; o que sai daqui vai sempre por uma carta com anexo.
CREATE TABLE auctions (
    id             BIGSERIAL PRIMARY KEY,
    seller_id      BIGINT NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    seller_account BIGINT NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    seller_name    TEXT NOT NULL,
    item_id        TEXT NOT NULL,
    count          INT NOT NULL CHECK (count > 0),
    enchant        SMALLINT NOT NULL DEFAULT 0 CHECK (enchant >= 0),
    price          BIGINT NOT NULL CHECK (price > 0),
    fee            BIGINT NOT NULL CHECK (fee >= 0),
    created_at     TIMESTAMPTZ NOT NULL,
    expires_at     TIMESTAMPTZ NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'sold', 'cancelled', 'expired')),
    buyer_id       BIGINT REFERENCES characters (id) ON DELETE SET NULL,
    buyer_name     TEXT NOT NULL DEFAULT '',
    commission     BIGINT NOT NULL DEFAULT 0 CHECK (commission >= 0),
    closed_at      TIMESTAMPTZ
);
-- Busca (por item e preço) e vencimento só olham os ativos.
CREATE INDEX auctions_active_item_idx ON auctions (item_id, price) WHERE status = 'active';
CREATE INDEX auctions_active_price_idx ON auctions (price, id) WHERE status = 'active';
CREATE INDEX auctions_expires_idx ON auctions (expires_at) WHERE status = 'active';
CREATE INDEX auctions_account_idx ON auctions (seller_account) WHERE status = 'active';
CREATE INDEX auctions_seller_idx ON auctions (seller_id, id DESC);

-- Anexo de uma carta do sistema (moedas e/ou um item). Retirar marca claimed_at na mesma
-- transação que grava a bolsa nova do personagem. Uma carta com anexo não retirado não
-- pode ser apagada (store.DeleteMail).
CREATE TABLE mail_attachments (
    mail_id    BIGINT PRIMARY KEY REFERENCES mail (id) ON DELETE CASCADE,
    to_id      BIGINT NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    item_id    TEXT NOT NULL DEFAULT '',
    count      INT NOT NULL DEFAULT 0 CHECK (count >= 0),
    enchant    SMALLINT NOT NULL DEFAULT 0 CHECK (enchant >= 0),
    gold       BIGINT NOT NULL DEFAULT 0 CHECK (gold >= 0),
    how        TEXT NOT NULL, -- telemetria ao retirar: auction_sale, auction_buy ou auction_return
    auction_id BIGINT,
    claimed_at TIMESTAMPTZ,
    CHECK (gold > 0 OR (item_id <> '' AND count > 0))
);
CREATE INDEX mail_attachments_open_idx ON mail_attachments (to_id) WHERE claimed_at IS NULL;

-- Operações da zona já aplicadas (anunciar, comprar, retirar): vêm em todo salvamento do
-- personagem até a zona saber que deram certo, e o banco aplica cada uma uma vez só.
CREATE TABLE auction_ops (
    character_id BIGINT NOT NULL REFERENCES characters (id) ON DELETE CASCADE,
    op_id        BIGINT NOT NULL,
    kind         TEXT NOT NULL,
    result       JSONB NOT NULL,
    at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, op_id)
);
CREATE INDEX auction_ops_at_idx ON auction_ops (at);

-- +goose Down
DROP TABLE auction_ops;
DROP TABLE mail_attachments;
DROP TABLE auctions;

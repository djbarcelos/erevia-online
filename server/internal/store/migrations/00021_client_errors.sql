-- +goose Up
-- Erros do cliente (navegador): window.onerror, promessas rejeitadas, perda do contexto
-- WebGL, mensagens do servidor que não decodificam e modelos que não carregam. Cada erro
-- distinto vira uma linha por versão do cliente (hash = mensagem + primeira linha da
-- stack); repetir soma a contagem e atualiza a última vez e o contexto (quem, onde, FPS e
-- ping da última ocorrência).
--
-- account_id e character_id não têm chave estrangeira, de propósito: gravar um erro não
-- pode travar (nem esperar) as linhas de contas e personagens, que a zona salva o tempo
-- todo, e o relatório não deve sumir junto com a conta. Ao apagar a conta, o
-- store.DeleteAccount zera esses dois campos (anonimiza) em vez de apagar o erro.
CREATE TABLE client_errors (
    id           BIGSERIAL PRIMARY KEY,
    hash         TEXT NOT NULL,
    version      TEXT NOT NULL DEFAULT '',
    kind         TEXT NOT NULL,
    message      TEXT NOT NULL,
    stack        TEXT NOT NULL DEFAULT '',
    count        BIGINT NOT NULL DEFAULT 1,
    first_seen   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen    TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_agent   TEXT NOT NULL DEFAULT '',
    account_id   BIGINT,
    character_id BIGINT,
    region       TEXT NOT NULL DEFAULT '',
    x            REAL,
    z            REAL,
    fps          REAL,
    ping         INT,
    -- Dados livres de quem relatou (ex.: invariantes do cliente), até 2 KB.
    extra        JSONB,
    UNIQUE (hash, version)
);
CREATE INDEX client_errors_last_seen_idx ON client_errors (last_seen DESC);
-- Apagar a conta procura as linhas dela.
CREATE INDEX client_errors_account_idx ON client_errors (account_id) WHERE account_id IS NOT NULL;

-- +goose Down
DROP TABLE client_errors;

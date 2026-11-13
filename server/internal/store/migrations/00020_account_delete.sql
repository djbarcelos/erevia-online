-- +goose Up
-- Apagar a conta (LGPD). Tudo o que depende da conta já some em cascata (identidades,
-- sessões, personagens, itens, armazém, participação em clãs), exceto um caso: o clã
-- apontava para o líder com ON DELETE CASCADE, então apagar o personagem líder apagaria o
-- clã inteiro e tiraria todos os outros membros. Agora o banco recusa apagar um líder:
-- quem apaga (store.DeleteAccount) passa a liderança ao próximo membro, ou desfaz o clã
-- se ele ficar vazio, antes de apagar.
ALTER TABLE clans
    DROP CONSTRAINT clans_leader_id_fkey,
    ADD CONSTRAINT clans_leader_id_fkey FOREIGN KEY (leader_id) REFERENCES characters (id) ON DELETE RESTRICT;

-- Apagar um personagem consulta clans por leader_id (a chave acima): sem índice, é uma
-- varredura da tabela inteira a cada personagem apagado.
CREATE INDEX clans_leader_idx ON clans (leader_id);

-- +goose Down
DROP INDEX clans_leader_idx;
ALTER TABLE clans
    DROP CONSTRAINT clans_leader_id_fkey,
    ADD CONSTRAINT clans_leader_id_fkey FOREIGN KEY (leader_id) REFERENCES characters (id) ON DELETE CASCADE;

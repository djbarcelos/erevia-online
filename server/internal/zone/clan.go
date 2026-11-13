package zone

import "erevia/server/internal/pb"

// clan é um clã em memória (a zona é a dona; o banco recebe as mudanças em ordem).
type clan struct {
	id      int64
	name    string
	notice  string
	leader  int64
	members map[int64]*clanMember // personagem → membro (online ou não)
}

type clanMember struct {
	id    int64
	name  string
	class string
	level int
	rank  int
}

// onClanCmd trata os comandos de clã (bloco 3).
func (z *Zone) onClanCmd(p *entity, m *pb.ClanCmd) {}

// clanJoin: p aceitou o convite de from para o clã clanID (bloco 3).
func (z *Zone) clanJoin(from, p *entity, clanID int64) {}

// clanChat manda a mensagem só aos membros do clã (bloco 3).
func (z *Zone) clanChat(p *entity, text string) {
	z.system(p, "Você não está em um clã.")
}

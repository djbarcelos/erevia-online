package zone

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

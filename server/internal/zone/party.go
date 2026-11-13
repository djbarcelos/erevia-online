package zone

// party é um grupo de jogadores (só em memória: sair do jogo = sair do grupo).
type party struct {
	id      uint32
	leader  uint32
	members []uint32 // ordem de entrada (o líder pode não ser o primeiro)
}

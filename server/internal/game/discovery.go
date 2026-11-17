package game

// DiscoveryState: as Pedras de Teleporte que o personagem já tocou a pé e a última cidade
// que visitou (para onde o Pergaminho de Retorno leva). Salvo em characters.discovered
// (migração 00036), junto com o personagem.
type DiscoveryState struct {
	// Stones: as chaves das pedras descobertas (zone.stoneDef.key).
	Stones []string `json:"stones,omitempty"`
	// Town: a chave da pedra da última cidade visitada ("" = Vel'Aran).
	Town string `json:"town,omitempty"`
}

// LegacyStoneKeys: as 5 pedras do mapa principal de antes do Brejo. Quem já tinha
// personagem quando as descobertas entraram ganhou todas (migração 00036); o personagem
// novo começa só com a da cidade (a primeira da lista).
var LegacyStoneKeys = []string{"cidade", "arredores", "ossos", "ermos", "vale"}

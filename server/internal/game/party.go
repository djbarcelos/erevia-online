package game

import "math"

// XP em grupo (tunnel#151). A zona decide quem conta (perto, vivo, ajudou); aqui ficam os
// números, que o simulador (internal/balance) também usa.
//
//   - Bônus: a XP total do abate sobe PartyBonusPerMember por membro que conta além do
//     primeiro (+10% com 2, +50% com 6) e é dividida pelo nível de cada um.
//   - Diferença de nível: se o maior e o menor nível dos que contam ficam a mais de
//     PartyFreeLevelGap um do outro, a XP do grupo inteiro cai (PartyGapMult). É o que
//     impede um nível alto de "carregar" um nível baixo.
const (
	PartyMaxMembers     = 6
	PartyBonusPerMember = 0.10
	PartyFreeLevelGap   = 8    // até 8 níveis entre o maior e o menor, sem corte
	PartyGapFirstMult   = 0.5  // 9 níveis: metade da XP
	PartyGapStep        = 0.1  // cada nível a mais tira mais 10 pontos
	PartyGapMinMult     = 0.1  // mínimo: 10% (13 níveis ou mais)
	PartyHelpSeconds    = 30.0 // ajudou (dano, cura ou buff) nos últimos 30 s
	PartyXPRange        = 40.0 // metros do monstro para contar
)

// PartyXPBonus: o multiplicador da XP total do abate com n membros que contam (1 a 6).
func PartyXPBonus(n int) float64 {
	n = min(max(n, 1), PartyMaxMembers)
	return 1 + PartyBonusPerMember*float64(n-1)
}

// PartyGapMult: quanto da XP do grupo sobra com a diferença gap entre o maior e o menor
// nível dos que contam. 0–8: tudo; 9: 50%; 10: 40%; 11: 30%; 12: 20%; 13 ou mais: 10%.
func PartyGapMult(gap int) float64 {
	if gap <= PartyFreeLevelGap {
		return 1
	}
	return math.Max(PartyGapMinMult, PartyGapFirstMult-PartyGapStep*float64(gap-PartyFreeLevelGap-1))
}

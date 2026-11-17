package game

import (
	"math"
	"slices"
)

// Missões diárias (tunnel#71): Cássio, o Arauto da Guarda, na praça, oferece 3 por dia, uma
// de cada vaga (coleta, entrega e ronda ou chefe), sorteadas do pool pelo personagem, pelo
// dia e pelo nível. Renovam à meia-noite de Brasília, com o mesmo relógio e o mesmo sorteio
// sem repetição dos contratos do nível máximo (ContractDay, NextDailyReset e pickNoRepeat,
// contracts.go).
//
// São para quem está do DailyMinLevel ao DailyMaxLevel: o nível máximo tem os contratos
// do quadro. E têm outro sabor que o dos contratos de caça (um por nível, do 5 ao 39, que só
// pedem abates): pegar materiais do saque, levar coisas a alguém, rondar um lugar do mapa ou
// enfrentar o Senhor dos Ossos.
//
// Recompensa: mais moedas, itens e reputação com a Guarda do que XP. A XP é a de
// DailyXPKills abates do nível do jogador (QuestReward.XPKills): 3 diárias por dia somam
// 2,5 min de caça, bem menos do que o tempo que elas levam, então a curva não encurta (a
// conta está no docs/progressao.md, "Missões de história e diárias").

const (
	DailyMinLevel = 5
	DailyMaxLevel = ContractMinLevel // 35: dali para cima são os contratos do quadro
	DailyXPKills  = 2.0              // a XP de uma diária, em abates do nível
	dailyRep      = 15               // reputação com a Guarda por diária
	dailyGoldLvl  = 35               // moedas por nível (o meio da faixa da diária)
)

// DailySlot é a vaga da diária no rol do Arauto.
type DailySlot int

const (
	DailyCollect DailySlot = iota + 1 // pegar materiais do saque
	DailyDeliver                      // levar itens a alguém
	DailyVenture                      // ronda (visitar lugares) ou chefe
)

// DailyVacancies: as vagas do rol, na ordem da conversa.
var DailyVacancies = []DailySlot{DailyCollect, DailyDeliver, DailyVenture}

func daily(id string, slot DailySlot, lo, hi int, title, text string, steps ...QuestStep) *QuestDef {
	mid := float64(lo+hi) / 2
	var bonus []QuestItem
	if lo >= 20 {
		bonus = items("potion_hp_l", 2)
	} else {
		bonus = items("potion_hp_s", 3)
	}
	return &QuestDef{
		ID: id, Title: title, Text: text, Steps: steps, Category: QuestDaily, Daily: slot,
		Giver: TalkHerald, Level: lo, MaxLevel: hi,
		Reward: QuestReward{XPKills: DailyXPKills, Gold: int64(math.Round(mid*dailyGoldLvl/10)) * 10, Items: bonus,
			Faction: FactionGuard, Rep: dailyRep},
	}
}

// dailyQuests: o pool. A ordem dentro de cada vaga é a do sorteio (mudar a ordem muda a
// diária de todo mundo; acrescente no fim).
var dailyQuests = []*QuestDef{
	// ---- Coleta ----
	daily("d_ossos", DailyCollect, 5, 11, "Ossos para a fogueira",
		"A Guarda queima os ossos dos mortos para que não levantem de novo. Pegue Ossos Velhos do saque dos esqueletos dos Arredores ou dos Campos.",
		collect("bone_old", 12, "Pegar Ossos Velhos do saque")),
	daily("d_cranios", DailyCollect, 8, 16, "Crânios que lembram",
		"Os escribas da Guarda fazem tinta de runa com o pó dos Crânios Rachados: dizem que o que resta de memória neles segura a tinta. Pegue alguns do saque.",
		collect("skull_cracked", 5, "Pegar Crânios Rachados do saque")),
	daily("d_frascos", DailyCollect, 13, 22, "Frascos de sombra",
		"A Essência Sombria dos Magos Esqueletos não pode ficar solta nos Ermos. Pegue-a do saque, e a Guarda cuida de enterrá-la em sal.",
		collect("shadow_essence", 3, "Pegar Essências Sombrias do saque")),
	daily("d_coracoes", DailyCollect, 20, 29, "Corações de pedra",
		"Os Núcleos Rúnicos dos golens seguram as pedras da muralha de Vel'Aran. Pegue Núcleos do saque no Vale das Pedras Vivas.",
		collect("rune_core", 6, "Pegar Núcleos Rúnicos do saque")),
	daily("d_nucleos_fundo", DailyCollect, 30, 35, "Núcleos do Fundo",
		"Os Golens Ancestrais carregam os Núcleos mais densos do Vale. A Guarda quer uma reserva deles antes que o Fundo trema de novo.",
		collect("rune_core", 8, "Pegar Núcleos Rúnicos do saque")),
	daily("d_velas", DailyCollect, 33, 35, "Velas apagadas",
		"Cada frasco de Essência Sombria tirado dos Devotos é uma vela a menos acesa na porta da Cripta. Pegue a essência do saque na Vigília Ossária.",
		collect("shadow_essence", 5, "Pegar Essências Sombrias do saque")),
	// ---- Entrega ----
	daily("d_remedio", DailyDeliver, 5, 10, "Remédio para o moinho",
		"Anselmo se machuca toda noite defendendo o trigal e não tem moedas para poção. Compre Poções de Vida Menores com a Mercadora Lívia e leve cinco a ele.",
		deliver("potion_hp_s", 5, TalkMiller, "Entregar 5 Poções de Vida Menores a Anselmo")),
	daily("d_cola", DailyDeliver, 5, 14, "Cola de osso",
		"O Ferreiro Bruno ferve Ossos Velhos para fazer a cola que prende o couro das armaduras da Guarda. Leve dez a ele.",
		deliver("bone_old", 10, TalkSmith, "Entregar 10 Ossos Velhos a Bruno")),
	daily("d_encomenda", DailyDeliver, 10, 20, "Encomenda da Mercadora",
		"A Mercadora Lívia tem um comprador para Crânios Rachados e paga a Guarda pela entrega. Leve quatro a ela.",
		deliver("skull_cracked", 4, TalkMerchant, "Entregar 4 Crânios Rachados a Lívia")),
	daily("d_suprimentos", DailyDeliver, 13, 22, "Suprimentos para os Ermos",
		"O batedor Dárcio passa semanas nos Ermos sem voltar à cidade. Compre Poções de Vida Maiores com Lívia e leve três a ele.",
		deliver("potion_hp_l", 3, TalkScout, "Entregar 3 Poções de Vida Maiores a Dárcio")),
	daily("d_tinta", DailyDeliver, 18, 28, "Tinta de sombra",
		"Maelis tece as runas do Vale com tinta de Essência Sombria. Leve duas essências ao acampamento do Vale.",
		deliver("shadow_essence", 2, TalkWeaver, "Entregar 2 Essências Sombrias a Maelis")),
	daily("d_muralha", DailyDeliver, 20, 35, "Pedra para a muralha",
		"Bruno remenda a muralha de Vel'Aran com Núcleos Rúnicos moídos na argamassa. Leve três a ele.",
		deliver("rune_core", 3, TalkSmith, "Entregar 3 Núcleos Rúnicos a Bruno")),
	daily("d_mantimentos", DailyDeliver, 28, 35, "Mantimentos para a Vigília",
		"O posto de Sor Bertrand, a sudoeste da Cripta, está sem poções. Compre Poções de Vida Maiores com Lívia e leve cinco a ele.",
		deliver("potion_hp_l", 5, TalkKnight, "Entregar 5 Poções de Vida Maiores a Bertrand")),
	// ---- Ronda ou chefe ----
	daily("d_ronda_arredores", DailyVenture, 5, 9, "Ronda dos Arredores",
		"A Guarda quer saber se a estrada dos Arredores está limpa. Passe pelo moinho, a leste, e pelo marco da estrada norte.",
		visit(spotMill, "Passar pelo Moinho dos Arredores"), visit(spotNorthMark, "Passar pelo marco da estrada norte")),
	daily("d_ronda_campos", DailyVenture, 6, 13, "Ronda dos Campos",
		"Nos Campos dos Ossos a Guarda vigia o Estandarte Caído e o marco da estrada leste. Passe pelos dois e veja se os mortos se juntaram.",
		visit(spotBanner, "Passar pelo Estandarte Caído"), visit(spotEastMark, "Passar pelo marco da estrada leste")),
	daily("d_fendas", DailyVenture, 13, 20, "As fendas dos Ermos",
		"As fendas dos Ermos se mexem. Passe pelo Passo de Brasaferro, no fim da estrada oeste, e pela Fenda do Sudoeste.",
		visit(spotPass, "Passar pelo Passo de Brasaferro"), visit(spotRift, "Passar pela Fenda do Sudoeste")),
	daily("d_arauto", DailyVenture, 18, 28, "Calar o arauto",
		"Onde o Senhor dos Ossos passa, os mortos levantam mais depressa. A Guarda quer o arauto do rei no chão hoje: junte um grupo na estrada sul.",
		kill("bone_lord", 1, "Derrotar o Senhor dos Ossos, na estrada sul")),
	daily("d_circulos", DailyVenture, 20, 30, "A ronda das pedras",
		"Maelis pede notícias dos círculos do Vale. Passe pelo Círculo Oeste e pelo Círculo Leste, entre os golens.",
		visit(spotWestRing, "Passar pelo Círculo Oeste"), visit(spotEastRing, "Passar pelo Círculo Leste")),
	daily("d_caidas", DailyVenture, 24, 33, "Pedras caídas",
		"Nas bordas do Vale há monólitos caídos que ninguém vigia. Passe pelos do oeste e pelos do leste e conte se algum se levantou.",
		visit(spotWestFall, "Passar pelas Pedras Caídas do Oeste"), visit(spotEastFall, "Passar pelas Pedras Caídas do Leste")),
	daily("d_fundo", DailyVenture, 30, 35, "O círculo inclinado",
		"O Irmão Gaspar quer saber, todo dia, para onde apontam as pedras do Círculo do Fundo. Vá até lá.",
		visit(spotDeepRing, "Chegar ao Círculo do Fundo")),
	daily("d_porta", DailyVenture, 33, 35, "Olhos na porta",
		"A Guarda manda alguém, todo dia, até a borda da Cripta, para ver se a porta continua fechada. Hoje é você.",
		visit(spotCrypt, "Chegar à borda da Cripta Esquecida")),
	// ---- Brejo de Lamagris (no fim, para não mudar o sorteio de ninguém) ----
	daily("d_juncos", DailyCollect, 14, 22, "Juncos-de-Prata",
		"O Junco-de-Prata só cresce nas margens do Brejo, e Tomé paga a Guarda por cada feixe: sem ele o Antídoto não sai. Pegue os juncos do saque dos bichos do pântano.",
		collect("silver_reed", 6, "Pegar Juncos-de-Prata do saque")),
	daily("d_antidoto", DailyDeliver, 14, 24, "Frascos para o posto",
		"Tomé gasta Antídoto como água, e o Ferreiro só faz com glândula e junco do Brejo. Faça o seu e leve três frascos ao posto do barqueiro.",
		deliver("potion_antidote", 3, TalkFerry, "Entregar 3 Antídotos a Tomé")),
	daily("d_ronda_brejo", DailyVenture, 14, 20, "Ronda do Brejo",
		"A Guarda quer saber se o cais do posto e a lagoa de Vau Velho seguem de pé. Passe pelo cais do barqueiro e pela lagoa onde só a torre da capela aparece.",
		visit(spotDock, "Passar pelo Cais do Barqueiro"), visit(spotMother, "Passar pela lagoa de Vau Velho")),
}

// dailyPool: as diárias da vaga para o nível.
func dailyPool(slot DailySlot, level int) []*QuestDef {
	var out []*QuestDef
	for _, q := range dailyQuests {
		if q.Daily == slot && level >= q.Level && level <= q.MaxLevel {
			out = append(out, q)
		}
	}
	return out
}

// DailyQuests: as diárias do personagem no dia, para o nível dele (uma por vaga, na ordem
// das vagas). Fora da faixa: nenhuma.
func DailyQuests(char uint32, day, level int) []*QuestDef {
	if level < DailyMinLevel || level > DailyMaxLevel {
		return nil
	}
	var out []*QuestDef
	for _, slot := range DailyVacancies {
		if q := pickNoRepeat(dailyPool(slot, level), char, dailySalt+uint64(slot), day); q != nil {
			out = append(out, q)
		}
	}
	return out
}

// dailySalt separa a semente das diárias da dos contratos (as vagas são números pequenos).
const dailySalt = 0x100

// DailyState é o que fica salvo das diárias (QuestState.Daily, em characters.quests).
type DailyState struct {
	Day   int      `json:"day,omitempty"`   // o dia do sorteio (ContractDay)
	Offer []string `json:"offer,omitempty"` // o sorteio do dia (fica o mesmo até a meia-noite)
	Done  []string `json:"done,omitempty"`  // as entregues hoje
}

// Refresh vira o dia: novo sorteio para o nível de agora e nenhuma entregue. No mesmo dia,
// quem chegou ao DailyMinLevel depois do sorteio vazio ganha o rol. Devolve se mudou.
func (s *DailyState) Refresh(char uint32, day, level int) bool {
	if s.Day == day && (len(s.Offer) > 0 || level < DailyMinLevel || level > DailyMaxLevel) {
		return false
	}
	if s.Day != day {
		s.Done = nil
	}
	s.Day, s.Offer = day, nil
	for _, q := range DailyQuests(char, day, level) {
		s.Offer = append(s.Offer, q.ID)
	}
	return true
}

// Offered: a diária está no rol de hoje.
func (s *DailyState) Offered(id string) bool { return s != nil && slices.Contains(s.Offer, id) }

// Delivered: a diária já foi entregue hoje.
func (s *DailyState) Delivered(id string) bool { return s != nil && slices.Contains(s.Done, id) }

// NormalDaily limpa o que veio do banco: ids que não são diárias e listas grandes demais.
func NormalDaily(s *DailyState) *DailyState {
	if s == nil {
		return nil
	}
	out := &DailyState{Day: s.Day}
	keep := func(ids []string) []string {
		var r []string
		for _, id := range ids {
			if q := Quests[id]; q != nil && q.Daily != 0 && !slices.Contains(r, id) && len(r) < len(DailyVacancies) {
				r = append(r, id)
			}
		}
		return r
	}
	out.Offer = keep(s.Offer)
	out.Done = keep(s.Done)
	return out
}

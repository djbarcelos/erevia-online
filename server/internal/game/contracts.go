package game

import (
	"math"
	"strings"
	"time"
)

// Contratos do nível máximo (tunnel#157): o quadro da praça oferece, a cada dia (meia-noite
// de Brasília), 3 diários, um de cada vaga (caça numa área, um chefe de campo e coleta de
// materiais), e 1 semanal maior (renova na segunda-feira, 0 h). O sorteio é determinístico
// por personagem e por dia, e nenhuma vaga repete o contrato do dia (ou da semana) anterior.
//
// Os diários seguem o "dia da região": num dia a caça é no Vale e a coleta e o chefe nos
// Ermos (dia do Vale); no outro, o contrário (dia dos Ermos). Os dias se alternam por
// personagem, então os três diários de hoje nunca são os de ontem e a caça de uma vaga não
// enche a coleta de outra (os Núcleos Rúnicos só caem dos golens, os crânios e ossos dos
// esqueletos). A conta do tempo por dia está no README ("Contratos e reputação").
//
// Concluir dá moedas, XP (no nível máximo, Maestria) e reputação com a facção do contrato
// (reputation.go). Tudo passa pela economia de sempre (zone/contracts.go).

// ContractMinLevel: nível para aceitar contratos ("perto do nível máximo"). Hoje o máximo é
// MaxLevel = 30 e a caça do fim é o Vale, a Cripta e os Ermos. Quando o nível máximo subir
// (tunnel#149, 40), rever este número junto com as áreas novas: o TestContractMinLevel só
// confere que ele continua perto do máximo e dentro da faixa do Vale.
const ContractMinLevel = 28

// ContractPvPMinLevel: no contrato de PvP, só conta vencer quem tem pelo menos este nível.
const ContractPvPMinLevel = 20

// ContractSlot é a vaga do contrato no quadro (os números são os do ContractSlot do
// protocolo).
type ContractSlot int

const (
	ContractHunt    ContractSlot = iota + 1 // diário: matar monstros de uma área
	ContractBoss                            // diário: chefe de campo
	ContractCollect                         // diário: coletar materiais do saque
	ContractWeekly                          // semanal
)

// DailySlots: as vagas dos diários, na ordem do quadro.
var DailySlots = []ContractSlot{ContractHunt, ContractBoss, ContractCollect}

// ContractGoal é o acontecimento que anda o contrato.
type ContractGoal int

const (
	CGoalKill    ContractGoal = iota + 1 // abater (com XP) monstros Target da Area, de nível ≥ MinLevel
	CGoalLoot                            // pegar do chão itens Target do saque de monstros
	CGoalRaid                            // participar (com dano) da morte do raid boss Target
	CGoalDefense                         // defender a cidade numa invasão vencida
	CGoalPvP                             // vencer jogadores (roxos ou com karma) de outra conta, nível ≥ ContractPvPMinLevel, cada um uma vez
)

// Áreas dos contratos de caça (onde o monstro nasceu, não onde morreu: a invasão traz
// esqueletos para perto da cidade e eles não contam).
const (
	AreaVale  = "vale"
	AreaErmos = "ermos"
)

// InContractArea diz se o ponto (onde o monstro nasce) fica na área. "" = qualquer lugar.
func InContractArea(area string, x, z float64) bool {
	switch area {
	case AreaVale: // o disco do Vale das Pedras Vivas (DefaultSpawns), com folga
		return math.Hypot(x, z+270) <= 60
	case AreaErmos: // o anel dos Ermos Partidos (r 116–140), com folga
		r := math.Hypot(x, z)
		return r >= 110 && r <= 146
	}
	return true
}

// ContractDef é um contrato do catálogo.
type ContractDef struct {
	ID     string
	Title  string
	Text   string // o que fazer, com um pouco de história
	Slot   ContractSlot
	Region string // dia da região (diários): AreaVale ou AreaErmos; "" = semanal
	Goal   ContractGoal
	Target string // ids separados por "|" (vazio = qualquer)
	Area   string // CGoalKill e CGoalLoot: onde o monstro nasceu ("" = qualquer lugar)
	// MinLevel: CGoalKill, só monstros deste nível para cima.
	MinLevel int
	Count    int
	Place    string // onde fica, para o jogador
	Faction  string // facção que dá a reputação (reputation.go)
	Gold     int64
	// XPShare: a XP da recompensa, em fração da barra do nível atual (XPToNext) ou, no
	// nível máximo, da barra de Maestria (MasteryPointXP).
	XPShare float64
	Rep     int
}

// Matches diz se o alvo do acontecimento vale para o contrato.
func (c *ContractDef) Matches(target string) bool {
	if c.Target == "" {
		return true
	}
	for _, t := range strings.Split(c.Target, "|") {
		if t == target {
			return true
		}
	}
	return false
}

// RewardXP: a XP da recompensa para quem está no nível level.
func (c *ContractDef) RewardXP(level int) int64 {
	bar := XPToNext(level)
	if level >= MasteryLevel {
		bar = MasteryPointXP
	}
	return int64(math.Round(float64(bar) * c.XPShare))
}

const (
	golems   = "stone_golem"
	ermosMob = "skeleton_warrior|skeleton_mage"
)

// ContractList: o catálogo. A ordem dentro de cada vaga e região é a do sorteio (mudar a
// ordem muda o contrato do dia de todo mundo; acrescente no fim).
var ContractList = []*ContractDef{
	// Caça — dia do Vale (Bastião Cinzento).
	{ID: "hunt_vale_golems", Title: "Pedras que andam", Slot: ContractHunt, Region: AreaVale,
		Text: "A Ordem do Bastião quer o caminho do Vale livre para as patrulhas. Abata Golens de Pedra, de qualquer tamanho.",
		Goal: CGoalKill, Target: golems, Area: AreaVale, Count: 100, Place: "Vale das Pedras Vivas", Faction: FactionBastion,
		Gold: 600, XPShare: 0.10, Rep: 120},
	{ID: "hunt_vale_deep", Title: "O fundo do Vale", Slot: ContractHunt, Region: AreaVale,
		Text: "Os golens mais antigos dormem no sul do Vale. A Ordem paga por cada um que não voltar a levantar.",
		Goal: CGoalKill, Target: golems, Area: AreaVale, MinLevel: 27, Count: 55, Place: "Vale das Pedras Vivas (sul, nível 27+)",
		Faction: FactionBastion, Gold: 650, XPShare: 0.10, Rep: 120},
	// Caça — dia dos Ermos (Guarda de Vel'Aran).
	{ID: "hunt_ermos_mages", Title: "Velas apagadas", Slot: ContractHunt, Region: AreaErmos,
		Text: "Os Esqueletos Magos dos Ermos acendem velas negras nas estradas. A Guarda quer as velas apagadas, e os magos também.",
		Goal: CGoalKill, Target: "skeleton_mage", Area: AreaErmos, MinLevel: 13, Count: 110, Place: "Ermos Partidos",
		Faction: FactionGuard, Gold: 600, XPShare: 0.10, Rep: 120},
	{ID: "hunt_ermos_patrol", Title: "Patrulha dos Ermos", Slot: ContractHunt, Region: AreaErmos,
		Text: "As caravanas não passam pelos Ermos com os mortos de pé. Limpe a estrada: guerreiros e magos contam.",
		Goal: CGoalKill, Target: ermosMob, Area: AreaErmos, MinLevel: 13, Count: 160, Place: "Ermos Partidos",
		Faction: FactionGuard, Gold: 650, XPShare: 0.10, Rep: 120},
	// Chefe de campo: o Senhor dos Ossos no dia do Vale (a coleta é nos Ermos e ele não deixa
	// crânio nem osso); os anciões do Vale no dia dos Ermos.
	{ID: "boss_bone_lord", Title: "A coroa partida", Slot: ContractBoss, Region: AreaVale,
		Text: "O Senhor dos Ossos volta a cada cinco minutos nos Ermos. A Guarda quer prova de que ele caiu duas vezes hoje.",
		Goal: CGoalKill, Target: "bone_lord", Count: 2, Place: "Senhor dos Ossos (Ermos, ao sul)",
		Faction: FactionGuard, Gold: 800, XPShare: 0.12, Rep: 150},
	{ID: "boss_golem_elders", Title: "Os anciões do Vale", Slot: ContractBoss, Region: AreaErmos,
		Text: "Os golens de nível 28 a 30 guardam o fundo do Vale. A Ordem quer vinte deles de volta ao chão.",
		Goal: CGoalKill, Target: golems, Area: AreaVale, MinLevel: 28, Count: 20, Place: "Vale das Pedras Vivas (fundo, nível 28+)",
		Faction: FactionBastion, Gold: 800, XPShare: 0.12, Rep: 150},
	// Coleta: nos Ermos no dia do Vale, no Vale no dia dos Ermos.
	{ID: "collect_skulls", Title: "Crânios para a muralha", Slot: ContractCollect, Region: AreaVale,
		Text: "A Guarda usa Crânios Rachados para contar os mortos que já não voltam. Pegue-os do saque dos esqueletos dos Ermos.",
		Goal: CGoalLoot, Target: "skull_cracked", Area: AreaErmos, Count: 22, Place: "Ermos Partidos",
		Faction: FactionGuard, Gold: 600, XPShare: 0.10, Rep: 120},
	{ID: "collect_bones", Title: "Ossos para o fogo", Slot: ContractCollect, Region: AreaVale,
		Text: "Osso queimado não levanta. A Guarda queima os Ossos Velhos que você pegar dos esqueletos dos Ermos.",
		Goal: CGoalLoot, Target: "bone_old", Area: AreaErmos, Count: 100, Place: "Ermos Partidos",
		Faction: FactionGuard, Gold: 600, XPShare: 0.10, Rep: 120},
	{ID: "collect_cores", Title: "Núcleos para a Ordem", Slot: ContractCollect, Region: AreaErmos,
		Text: "Os monges do Bastião estudam os Núcleos Rúnicos que ainda pulsam. Pegue-os do saque dos golens.",
		Goal: CGoalLoot, Target: "rune_core", Area: AreaVale, Count: 32, Place: "Vale das Pedras Vivas",
		Faction: FactionBastion, Gold: 600, XPShare: 0.10, Rep: 120},
	// Semanais.
	{ID: "weekly_ozrhal", Title: "O Rei Ossário", Slot: ContractWeekly,
		Text: "A Ordem do Bastião vigia a Cripta desde a fundação. Participe da queda de Ozrhal, o Rei Ossário.",
		Goal: CGoalRaid, Target: "ozrhal", Count: 1, Place: "Cripta Esquecida (nordeste)",
		Faction: FactionBastion, Gold: 4000, XPShare: 0.5, Rep: 500},
	{ID: "weekly_bone_lord", Title: "A queda do Senhor", Slot: ContractWeekly,
		Text: "Oito vezes nesta semana: a Guarda quer o Senhor dos Ossos sem tempo de juntar os pedaços.",
		Goal: CGoalKill, Target: "bone_lord", Count: 8, Place: "Senhor dos Ossos (Ermos, ao sul)",
		Faction: FactionGuard, Gold: 3500, XPShare: 0.5, Rep: 500},
	{ID: "weekly_invasion", Title: "Muralhas de pé", Slot: ContractWeekly,
		Text: "Quando os esqueletos marcham para a praça, a Guarda conta quem ficou. Defenda a cidade em duas invasões vencidas.",
		Goal: CGoalDefense, Count: 2, Place: "Vel'Aran (à noite)",
		Faction: FactionGuard, Gold: 3500, XPShare: 0.5, Rep: 500},
	{ID: "weekly_pvp", Title: "Justiça da Guarda", Slot: ContractWeekly,
		Text: "Vença três jogadores em combate (de nome roxo ou com karma, do nível 20 para cima, de outra conta; cada um conta uma vez).",
		Goal: CGoalPvP, Count: 3, Place: "Fora da cidade",
		Faction: FactionGuard, Gold: 3500, XPShare: 0.5, Rep: 500},
	{ID: "weekly_vale_purge", Title: "Expurgo do Vale", Slot: ContractWeekly,
		Text: "Uma semana de caça para a Ordem: trezentos golens do Vale das Pedras Vivas.",
		Goal: CGoalKill, Target: golems, Area: AreaVale, Count: 300, Place: "Vale das Pedras Vivas",
		Faction: FactionBastion, Gold: 4000, XPShare: 0.5, Rep: 500},
}

// Contracts: os contratos por id.
var Contracts = func() map[string]*ContractDef {
	m := make(map[string]*ContractDef, len(ContractList))
	for _, c := range ContractList {
		m[c.ID] = c
	}
	return m
}()

// contractPool: os contratos de uma vaga num dia da região (semanal: Region "").
func contractPool(slot ContractSlot, region string) []*ContractDef {
	var out []*ContractDef
	for _, c := range ContractList {
		if c.Slot == slot && c.Region == region {
			out = append(out, c)
		}
	}
	return out
}

// ---------- Dias e semanas (Brasília) ----------

// ContractDay: o dia de t no fuso loc (o mesmo número de MasteryDay: dias desde 1970).
func ContractDay(t time.Time, loc *time.Location) int { return MasteryDay(t, loc) }

// ContractWeek: a semana do dia (começa na segunda-feira; 1970-01-05 foi segunda).
func ContractWeek(day int) int { return floorDiv(day-4, 7) }

// DayStart: a meia-noite do dia no fuso loc.
func DayStart(day int, loc *time.Location) time.Time {
	y, m, d := time.Unix(int64(day)*86400, 0).UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

// NextDailyReset: a próxima meia-noite (os diários renovam).
func NextDailyReset(day int, loc *time.Location) time.Time { return DayStart(day+1, loc) }

// NextWeeklyReset: a próxima segunda-feira, 0 h (o semanal renova).
func NextWeeklyReset(day int, loc *time.Location) time.Time {
	return DayStart(4+7*(ContractWeek(day)+1), loc)
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// ---------- Sorteio ----------

// mix64: splitmix64 (o sorteio não depende da versão do math/rand).
func mix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

func contractSeed(char uint32, slot ContractSlot, period int) uint64 {
	return mix64(uint64(char)<<32 ^ uint64(slot)<<24 ^ mix64(uint64(int64(period))))
}

// DayRegion: o dia da região do personagem (alterna todo dia; quem começa com qual sai do
// id do personagem).
func DayRegion(char uint32, day int) string {
	if (day+int(mix64(uint64(char))&1))%2 == 0 {
		return AreaVale
	}
	return AreaErmos
}

// DailyContracts: os 3 diários do personagem no dia, na ordem das vagas (caça, chefe,
// coleta). Dois dias seguidos são de regiões diferentes, então nenhuma vaga repete: o
// sorteio dentro da região roda pelos dias daquela região (a cada 2 dias).
func DailyContracts(char uint32, day int) []*ContractDef {
	region := DayRegion(char, day)
	out := make([]*ContractDef, 0, len(DailySlots))
	for _, slot := range DailySlots {
		pool := contractPool(slot, region)
		if len(pool) == 0 {
			continue
		}
		turn := floorDiv(day, 2) // os dias desta região, um a um
		out = append(out, pool[int(uint64(int64(turn)+int64(contractSeed(char, slot, 0)%1024))%uint64(len(pool)))])
	}
	return out
}

// WeeklyContract: o semanal do personagem na semana, sem repetir o da semana anterior.
func WeeklyContract(char uint32, week int) *ContractDef {
	return pickNoRepeat(contractPool(ContractWeekly, ""), char, ContractWeekly, week)
}

// OfferedContracts: o que o quadro oferece ao personagem no dia (diários e o semanal).
func OfferedContracts(char uint32, day int) []*ContractDef {
	out := DailyContracts(char, day)
	if w := WeeklyContract(char, ContractWeek(day)); w != nil {
		out = append(out, w)
	}
	return out
}

// pickNoRepeat sorteia da lista sem repetir o período anterior, sem depender de nenhum
// estado: os períodos andam em ciclos de len(pool), e cada ciclo é uma permutação própria
// (embaralhada com a semente do personagem). Dentro do ciclo nada repete; na emenda, se o
// primeiro do ciclo for o último do anterior, os dois primeiros trocam de lugar (com 3 ou
// mais opções a troca nunca mexe no último do ciclo, então a conta fecha). Com 2 opções,
// alterna.
func pickNoRepeat(pool []*ContractDef, char uint32, slot ContractSlot, period int) *ContractDef {
	n := len(pool)
	switch n {
	case 0:
		return nil
	case 1:
		return pool[0]
	case 2:
		return pool[(period+int(contractSeed(char, slot, 0)&1))%2&1]
	}
	cycle := floorDiv(period, n)
	pos := period - cycle*n
	perm := contractPerm(n, contractSeed(char, slot, cycle))
	if pos <= 1 && perm[0] == contractPerm(n, contractSeed(char, slot, cycle-1))[n-1] {
		perm[0], perm[1] = perm[1], perm[0]
	}
	return pool[perm[pos]]
}

// contractPerm: uma permutação de 0..n-1 (Fisher–Yates com splitmix64).
func contractPerm(n int, seed uint64) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	s := seed
	for i := n - 1; i > 0; i-- {
		s = mix64(s)
		j := int(s % uint64(i+1))
		p[i], p[j] = p[j], p[i]
	}
	return p
}

// ---------- Estado salvo ----------

// ContractProgress: um contrato aceito.
type ContractProgress struct {
	N    int  `json:"n,omitempty"`
	Done bool `json:"done,omitempty"` // concluído (a recompensa já saiu)
	// Seen: CGoalPvP, os personagens que já contaram (cada um vale uma vez).
	Seen []uint32 `json:"seen,omitempty"`
}

// ContractState é o que fica salvo no personagem (characters.contracts).
type ContractState struct {
	Day  int `json:"day,omitempty"`  // dia dos diários em Taken
	Week int `json:"week,omitempty"` // semana do semanal em Taken
	// Taken: os contratos aceitos hoje (e o semanal desta semana), por id.
	Taken map[string]*ContractProgress `json:"taken,omitempty"`
}

// Refresh vira o dia e a semana: os diários de ontem e o semanal da semana passada saem
// (aceitos ou não), e o que não está mais no quadro de hoje também. Devolve se mudou.
func (s *ContractState) Refresh(char uint32, day int) bool {
	week := ContractWeek(day)
	changed := false
	if s.Day != day || s.Week != week {
		offered := map[string]bool{}
		for _, c := range OfferedContracts(char, day) {
			offered[c.ID] = true
		}
		for id := range s.Taken {
			def := Contracts[id]
			stale := def == nil || !offered[id] ||
				(def.Slot != ContractWeekly && s.Day != day) || (def.Slot == ContractWeekly && s.Week != week)
			if stale {
				delete(s.Taken, id)
			}
		}
		s.Day, s.Week, changed = day, week, true
	}
	return changed
}

// NormalContracts limpa o que veio do banco: contratos que não existem, progresso fora da
// faixa e listas grandes demais.
func NormalContracts(st ContractState) ContractState {
	out := ContractState{Day: st.Day, Week: st.Week}
	for id, p := range st.Taken {
		def := Contracts[id]
		if def == nil || p == nil {
			continue
		}
		if out.Taken == nil {
			out.Taken = map[string]*ContractProgress{}
		}
		q := &ContractProgress{N: min(max(p.N, 0), def.Count), Done: p.Done}
		if q.Done {
			q.N = def.Count
		}
		if def.Goal == CGoalPvP && len(p.Seen) > 0 {
			q.Seen = append([]uint32(nil), p.Seen[:min(len(p.Seen), def.Count)]...)
		}
		out.Taken[id] = q
	}
	return out
}

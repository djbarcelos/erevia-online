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
// Os diários usam as áreas do 30 ao 40 (tunnel#149): o Fundo do Vale, a Vigília Ossária e
// os Confins Partidos. Cada dia tem uma "região do dia", que roda pelas três (a ordem de
// começo sai do personagem): a caça é na região do dia, o chefe na seguinte e a coleta na
// outra. Como as três vagas ficam sempre em áreas diferentes e a coleta só conta o saque de
// monstros que nasceram na área dela, a caça e o chefe nunca enchem a coleta. E como dois
// dias seguidos têm regiões diferentes, nenhuma vaga repete a de ontem. A conta do tempo
// por dia está no README ("Contratos e reputação").
//
// Concluir dá moedas, XP (no nível máximo, Maestria) e reputação com a facção do contrato
// (reputation.go). Tudo passa pela economia de sempre (zone/contracts.go).

// ContractMinLevel: nível para aceitar contratos ("perto do nível máximo"): os 5 últimos
// níveis, a faixa dos Confins Partidos (35–40). Sai de MaxLevel: se o máximo subir de novo,
// o limiar sobe junto (e o TestContractTargetsNotTrivial cobra áreas à altura).
const ContractMinLevel = MaxLevel - 5

// ContractPvPMinLevel: no contrato de PvP, só conta vencer quem tem pelo menos este nível.
const ContractPvPMinLevel = MaxLevel - 10

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
	CGoalLoot                            // pegar do chão itens Target do saque de monstros da Area
	CGoalRaid                            // participar (com dano) da morte do raid boss Target
	CGoalDefense                         // defender a cidade numa invasão vencida
	CGoalPvP                             // vencer jogadores (roxos ou com karma) de outra conta, nível ≥ ContractPvPMinLevel, cada um uma vez
)

// Áreas dos contratos: as regiões do 30 ao 40 de DefaultSpawns (o nome é o Region dos
// spawns). Conta onde o monstro nasceu, não onde morreu.
const (
	AreaFundo   = "Fundo do Vale"
	AreaVigil   = "Vigília Ossária"
	AreaConfins = "Confins Partidos"
)

// ContractRegions: a ordem da rotação da região do dia.
var ContractRegions = []string{AreaFundo, AreaVigil, AreaConfins}

// contractAreaMargin: folga (m) em volta dos discos das áreas.
const contractAreaMargin = 6.0

// contractAreas: os discos de cada área, tirados de DefaultSpawns.
var contractAreas = func() map[string][]Spawn {
	m := map[string][]Spawn{}
	for _, s := range DefaultSpawns() {
		if s.Region != "" && s.Area {
			m[s.Region] = append(m[s.Region], s)
		}
	}
	return m
}()

// InContractArea diz se o ponto (onde o monstro nasce) fica na área. "" = qualquer lugar.
func InContractArea(area string, x, z float64) bool {
	if area == "" {
		return true
	}
	for _, s := range contractAreas[area] {
		if s.Contains(x, z, contractAreaMargin) {
			return true
		}
	}
	return false
}

// ContractDef é um contrato do catálogo.
type ContractDef struct {
	ID     string
	Title  string
	Text   string // o que fazer, com um pouco de história
	Slot   ContractSlot
	Region string // diários: a região do dia em que ele sai (ContractRegions); "" = semanal
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
	vigilMobs   = "ossuary_knight|ossuary_devotee"
	confinsMobs = "broken_reaper|broken_arcanist"
	eliteMobs   = "ancient_golem|" + vigilMobs + "|" + confinsMobs
)

// ContractList: o catálogo. A ordem dentro de cada vaga e região é a do sorteio (mudar a
// ordem muda o contrato do dia de todo mundo; acrescente no fim). A caça e o chefe dão
// reputação com a facção da área (Fundo e Vigília: o Bastião; Confins: a Guarda); a coleta é
// sempre da Guarda, que abastece a muralha.
var ContractList = []*ContractDef{
	// ---- Dia do Fundo do Vale: caça no Fundo, chefe na Vigília, coleta nos Confins ----
	{ID: "hunt_fundo_golems", Title: "Musgo e runas", Slot: ContractHunt, Region: AreaFundo,
		Text: "Os Golens Ancestrais do Fundo do Vale acordam com o tremor da Cripta. A Ordem do Bastião quer o sul do Vale quieto de novo.",
		Goal: CGoalKill, Target: "ancient_golem", Area: AreaFundo, Count: 70, Place: "Fundo do Vale (sul do Vale)",
		Faction: FactionBastion, Gold: 2000, XPShare: 0.025, Rep: 120},
	{ID: "hunt_fundo_deep", Title: "Tremor no Fundo", Slot: ContractHunt, Region: AreaFundo,
		Text: "Os golens mais antigos, do nível 32 para cima, são os que racham o chão. A Ordem paga por cada um deles.",
		Goal: CGoalKill, Target: "ancient_golem", Area: AreaFundo, MinLevel: 32, Count: 45, Place: "Fundo do Vale (nível 32+)",
		Faction: FactionBastion, Gold: 2100, XPShare: 0.025, Rep: 120},
	{ID: "boss_vigil_captains", Title: "Os capitães da Vigília", Slot: ContractBoss, Region: AreaFundo,
		Text: "Os Cavaleiros e Devotos de nível 36 para cima comandam a Vigília Ossária. A Ordem quer trinta deles fora do caminho da Cripta.",
		Goal: CGoalKill, Target: vigilMobs, Area: AreaVigil, MinLevel: 36, Count: 30, Place: "Vigília Ossária (ao lado da Cripta, nível 36+)",
		Faction: FactionBastion, Gold: 2500, XPShare: 0.03, Rep: 150},
	{ID: "collect_confins_essence", Title: "Essência dos cantos", Slot: ContractCollect, Region: AreaFundo,
		Text: "A Guarda queima a Essência Sombria que escorre da Partição. Pegue-a do saque dos Ceifadores e Arcanistas dos Confins.",
		Goal: CGoalLoot, Target: "shadow_essence", Area: AreaConfins, Count: 18, Place: "Confins Partidos (os cantos do mapa)",
		Faction: FactionGuard, Gold: 2000, XPShare: 0.025, Rep: 120},
	{ID: "collect_confins_cores", Title: "Núcleos rachados", Slot: ContractCollect, Region: AreaFundo,
		Text: "Os Núcleos Rúnicos que os Partidos carregam estão rachados, mas a Guarda ainda os quer. Pegue-os do saque nos Confins.",
		Goal: CGoalLoot, Target: "rune_core", Area: AreaConfins, Count: 8, Place: "Confins Partidos (os cantos do mapa)",
		Faction: FactionGuard, Gold: 2000, XPShare: 0.025, Rep: 120},
	// ---- Dia da Vigília Ossária: caça na Vigília, chefe nos Confins, coleta no Fundo ----
	{ID: "hunt_vigil_guard", Title: "A guarda do selo", Slot: ContractHunt, Region: AreaVigil,
		Text: "Os Cavaleiros e os Devotos do Ossário cantam do lado de fora da Cripta. A Ordem quer o canto calado.",
		Goal: CGoalKill, Target: vigilMobs, Area: AreaVigil, Count: 110, Place: "Vigília Ossária (ao lado da Cripta)",
		Faction: FactionBastion, Gold: 2000, XPShare: 0.025, Rep: 120},
	{ID: "hunt_vigil_devotees", Title: "Cânticos calados", Slot: ContractHunt, Region: AreaVigil,
		Text: "São os Devotos que alimentam o selo rachado. A Ordem paga por cada um que não cantar mais.",
		Goal: CGoalKill, Target: "ossuary_devotee", Area: AreaVigil, Count: 55, Place: "Vigília Ossária (ao lado da Cripta)",
		Faction: FactionBastion, Gold: 2100, XPShare: 0.025, Rep: 120},
	{ID: "boss_confins_reapers", Title: "Os ceifadores do canto", Slot: ContractBoss, Region: AreaVigil,
		Text: "Nos Confins, os Partidos de nível 38 a 40 são os que abrem a rachadura. A Guarda quer vinte deles no chão.",
		Goal: CGoalKill, Target: confinsMobs, Area: AreaConfins, MinLevel: 38, Count: 20, Place: "Confins Partidos (nível 38+)",
		Faction: FactionGuard, Gold: 2500, XPShare: 0.03, Rep: 150},
	{ID: "collect_fundo_cores", Title: "Núcleos para a muralha", Slot: ContractCollect, Region: AreaVigil,
		Text: "Os Núcleos Rúnicos dos golens ancestrais seguram as pedras da muralha. Pegue-os do saque no Fundo do Vale.",
		Goal: CGoalLoot, Target: "rune_core", Area: AreaFundo, Count: 50, Place: "Fundo do Vale (sul do Vale)",
		Faction: FactionGuard, Gold: 2000, XPShare: 0.025, Rep: 120},
	// ---- Dia dos Confins Partidos: caça nos Confins, chefe no Fundo, coleta na Vigília ----
	{ID: "hunt_confins_broken", Title: "Rachaduras", Slot: ContractHunt, Region: AreaConfins,
		Text: "Os Ceifadores e os Arcanistas Partidos saem das rachaduras dos cantos do mapa. A Guarda quer os cantos limpos.",
		Goal: CGoalKill, Target: confinsMobs, Area: AreaConfins, Count: 60, Place: "Confins Partidos (os cantos do mapa)",
		Faction: FactionGuard, Gold: 2000, XPShare: 0.025, Rep: 120},
	{ID: "hunt_confins_arcanists", Title: "Gelo nos cantos", Slot: ContractHunt, Region: AreaConfins,
		Text: "O gelo dos Arcanistas Partidos fecha as estradas. A Guarda paga por cada um deles.",
		Goal: CGoalKill, Target: "broken_arcanist", Area: AreaConfins, Count: 30, Place: "Confins Partidos (os cantos do mapa)",
		Faction: FactionGuard, Gold: 2100, XPShare: 0.025, Rep: 120},
	{ID: "boss_fundo_elders", Title: "Os anciões do Fundo", Slot: ContractBoss, Region: AreaConfins,
		Text: "Os Golens Ancestrais de nível 34 e 35 são os mais velhos do Vale. A Ordem quer quinze deles de volta ao chão.",
		Goal: CGoalKill, Target: "ancient_golem", Area: AreaFundo, MinLevel: 34, Count: 15, Place: "Fundo do Vale (nível 34+)",
		Faction: FactionBastion, Gold: 2500, XPShare: 0.03, Rep: 150},
	{ID: "collect_vigil_skulls", Title: "Crânios do Ossário", Slot: ContractCollect, Region: AreaConfins,
		Text: "A Guarda conta os mortos que já não voltam pelos crânios. Pegue os Crânios Rachados do saque na Vigília Ossária.",
		Goal: CGoalLoot, Target: "skull_cracked", Area: AreaVigil, Count: 50, Place: "Vigília Ossária (ao lado da Cripta)",
		Faction: FactionGuard, Gold: 2000, XPShare: 0.025, Rep: 120},
	{ID: "collect_vigil_essence", Title: "Velas de sebo negro", Slot: ContractCollect, Region: AreaConfins,
		Text: "Os Devotos alimentam o selo com Essência Sombria. A Guarda a quer longe da Cripta: pegue-a do saque na Vigília.",
		Goal: CGoalLoot, Target: "shadow_essence", Area: AreaVigil, Count: 28, Place: "Vigília Ossária (ao lado da Cripta)",
		Faction: FactionGuard, Gold: 2000, XPShare: 0.025, Rep: 120},
	// ---- Semanais ----
	{ID: "weekly_ozrhal", Title: "O Rei Ossário", Slot: ContractWeekly,
		Text: "A Ordem do Bastião vigia a Cripta desde a fundação. Participe da queda de Ozrhal, o Rei Ossário.",
		Goal: CGoalRaid, Target: "ozrhal", Count: 1, Place: "Cripta Esquecida (nordeste)",
		Faction: FactionBastion, Gold: 12000, XPShare: 0.1, Rep: 500},
	{ID: "weekly_rares", Title: "Caçador de raros", Slot: ContractWeekly,
		Text: "Ossudo, o Capitão, Vossk, Ômrun: a Guarda quer saber de cada raro que aparece. Derrote dois monstros raros nesta semana.",
		Goal: CGoalKill, Target: RareIDs(), Count: 2, Place: "Onde um raro aparecer (o chat avisa quem está perto)",
		Faction: FactionGuard, Gold: 10000, XPShare: 0.1, Rep: 500},
	{ID: "weekly_invasion", Title: "Muralhas de pé", Slot: ContractWeekly,
		Text: "Quando os esqueletos marcham para a praça, a Guarda conta quem ficou. Defenda a cidade em duas invasões vencidas.",
		Goal: CGoalDefense, Count: 2, Place: "Vel'Aran (à noite)",
		Faction: FactionGuard, Gold: 10000, XPShare: 0.1, Rep: 500},
	{ID: "weekly_pvp", Title: "Justiça da Guarda", Slot: ContractWeekly,
		Text: "Vença três jogadores em combate (de nome roxo ou com karma, do nível 30 para cima, de outra conta; cada um conta uma vez).",
		Goal: CGoalPvP, Count: 3, Place: "Fora da cidade",
		Faction: FactionGuard, Gold: 10000, XPShare: 0.1, Rep: 500},
	{ID: "weekly_purge", Title: "Expurgo do Ossário", Slot: ContractWeekly,
		Text: "Uma semana de caça para a Ordem: duzentos e cinquenta Golens Ancestrais, Cavaleiros, Devotos, Ceifadores ou Arcanistas.",
		Goal: CGoalKill, Target: eliteMobs, Count: 250, Place: "Fundo do Vale, Vigília Ossária e Confins Partidos",
		Faction: FactionBastion, Gold: 12000, XPShare: 0.1, Rep: 500},
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

// DayRegion: a região do dia do personagem. Roda pelas três de ContractRegions, um passo
// por dia (onde começa sai do id do personagem): dois dias seguidos nunca têm a mesma.
func DayRegion(char uint32, day int) string {
	n := len(ContractRegions)
	off := int(mix64(uint64(char)) % uint64(n))
	return ContractRegions[((day+off)%n+n)%n]
}

// DailyContracts: os 3 diários do personagem no dia, na ordem das vagas (caça, chefe,
// coleta). Dois dias seguidos são de regiões diferentes, então nenhuma vaga repete: o
// sorteio dentro da região roda pelos dias daquela região (a cada 3 dias).
func DailyContracts(char uint32, day int) []*ContractDef {
	region := DayRegion(char, day)
	turn := floorDiv(day, len(ContractRegions)) // os dias desta região, um a um
	out := make([]*ContractDef, 0, len(DailySlots))
	for _, slot := range DailySlots {
		pool := contractPool(slot, region)
		if len(pool) == 0 {
			continue
		}
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

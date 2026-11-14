package game

import (
	"fmt"
	"math"
	"time"
)

// Maestria: a progressão depois do nível máximo, nos moldes do Champion System do ESO, do
// AA do EverQuest e do Paragon do Diablo 3. No nível máximo (40) a XP (de monstro, missão e
// evento, com as regras de sempre) enche a barra de Maestria, e cada barra cheia dá 1 ponto
// para a árvore de 3 ramos (Caça, Sobrevivência e Utilidade).
//
// Quem ganhou pontos quando o máximo era 30 fica com eles e continua usando a árvore (gastar
// e redistribuir) do 30 ao 39; só não ganha pontos novos antes do 40, porque a XP volta a
// subir nível (MasteryState.Usable).
//
// A árvore é mais horizontal que vertical: os nós dão opções (poção, ouro, viagem, controle,
// regeneração) e quase nada de força. Tudo o que mexe no combate (dano, defesa, vida,
// crítico e velocidade de ataque), somado com a árvore cheia, fica em no máximo
// MasteryCombatCap (o teste TestMasteryCombatCap trava isso).
//
// Bônus diário (como o Enlightenment do ESO): a primeira barra de cada dia (Brasília) sai
// MasteryRestedMult× mais rápido, e quem fica sem jogar guarda até MasteryRestedDays dias.

const (
	// MasteryLevel: a Maestria abre no nível máximo.
	MasteryLevel = MaxLevel
	// MasteryPointXP: XP de uma barra (1 ponto), igual a XPToNext(MaxLevel−1) = 45,2
	// milhões: um ponto pede o mesmo esforço que o 39 → 40. É uma constante (e não a função)
	// para poder ser usada em constantes; o TestMasteryPointXP confere que bate com a curva.
	MasteryPointXP int64 = 45_200_000
	// MasteryRestedMult: o bônus diário enche a barra 4× mais rápido.
	MasteryRestedMult = 4
	// MasteryRestedDays: dias de bônus que dá para guardar (1 barra por dia).
	MasteryRestedDays = 7
	// MasteryNodeMax: pontos por nó.
	MasteryNodeMax = 5
	// MasteryCombatCap: teto da soma dos bônus de combate com a árvore inteira (5%).
	MasteryCombatCap = 0.05
	// MasterySpeedCap: teto da velocidade fora de combate (pouca, e com teto).
	MasterySpeedCap = 0.04
)

// MasteryRankShare: quanto do valor total do nó vem de cada ponto (retorno decrescente:
// o 1º ponto vale 5× o 5º).
var MasteryRankShare = [MasteryNodeMax]float64{0.40, 0.25, 0.15, 0.12, 0.08}

// MasteryStat é o efeito de um nó.
type MasteryStat int

const (
	MStatNone      MasteryStat = iota
	MStatPotion                // poções curam mais (+%)
	MStatGold                  // moedas dos monstros (+%)
	MStatMaterial              // chance de +1 material no saque
	MStatDamage                // dano (+%) — combate
	MStatCCResist              // duração de atordoamento e lentidão (−%)
	MStatDeathLoss             // perda da barra ao morrer (−%)
	MStatCalmRegen             // regeneração de vida e mana fora de combate (+%)
	MStatMaxHP                 // vida máxima (+%) — combate
	MStatCalmSpeed             // velocidade fora de combate (+%)
	MStatTeleport              // preço das Pedras de Teleporte (−%)
	MStatChannel               // tempo para viajar (pedra e Pergaminho de Retorno, −%)
	MStatReturnCD              // recarga do Pergaminho de Retorno (−%)
	mstatCount
)

// Combat: o efeito entra no teto de combate (dano, defesa, vida, crítico, velocidade de
// ataque). Os outros não deixam ninguém mais forte numa luta.
func (s MasteryStat) Combat() bool { return s == MStatDamage || s == MStatMaxHP }

// MasteryBranch é um ramo da árvore.
type MasteryBranch struct {
	Name, Description, Color string
}

// MasteryNode é um nó da árvore (até MasteryNodeMax pontos).
type MasteryNode struct {
	ID     string
	Name   string
	Branch int
	Stat   MasteryStat
	Total  float64 // valor com todos os pontos (0,2 = 20%)
	Req    int     // pontos gastos no ramo antes de abrir
	Icon   string  // glifo do cliente (GLYPHS em client/src/ui/art.ts)
	Format string  // texto do efeito, com %s = o valor ("+20%")
}

var MasteryBranches = []MasteryBranch{
	{Name: "Caça", Description: "Poções, moedas e materiais", Color: "#d9a441"},
	{Name: "Sobrevivência", Description: "Controle, morte e fôlego", Color: "#5fae6e"},
	{Name: "Utilidade", Description: "Viagem e deslocamento", Color: "#5b8fd6"},
}

// MasteryNodes: 12 nós, 4 por ramo (60 pontos para a árvore inteira).
var MasteryNodes = []*MasteryNode{
	// Caça
	{ID: "field_alchemy", Name: "Alquimia de Campo", Branch: 0, Stat: MStatPotion, Total: 0.20, Icon: "flask",
		Format: "Cura das poções: %s."},
	{ID: "full_purse", Name: "Bolsa Farta", Branch: 0, Stat: MStatGold, Total: 0.15, Icon: "crown",
		Format: "Moedas dos monstros: %s."},
	{ID: "keen_eye", Name: "Olho do Coletor", Branch: 0, Stat: MStatMaterial, Total: 0.15, Req: 5, Icon: "eye",
		Format: "Chance de 1 material a mais no saque de monstros comuns: %s."},
	{ID: "hunter_instinct", Name: "Instinto do Caçador", Branch: 0, Stat: MStatDamage, Total: 0.02, Req: 10, Icon: "daggers",
		Format: "Dano: %s."},
	// Sobrevivência
	{ID: "steadfast", Name: "Firmeza", Branch: 1, Stat: MStatCCResist, Total: 0.25, Icon: "shield",
		Format: "Duração de atordoamento e lentidão em você: %s."},
	{ID: "second_chance", Name: "Segunda Chance", Branch: 1, Stat: MStatDeathLoss, Total: 0.50, Icon: "skull",
		Format: "XP da barra perdida ao morrer: %s."},
	{ID: "second_wind", Name: "Fôlego", Branch: 1, Stat: MStatCalmRegen, Total: 0.40, Req: 5, Icon: "drop",
		Format: "Regeneração de vida e mana fora de combate: %s."},
	{ID: "tanned_hide", Name: "Couro Curtido", Branch: 1, Stat: MStatMaxHP, Total: 0.015, Req: 10, Icon: "heart",
		Format: "Vida máxima: %s."},
	// Utilidade
	{ID: "light_step", Name: "Passo Leve", Branch: 2, Stat: MStatCalmSpeed, Total: MasterySpeedCap, Icon: "dash",
		Format: "Velocidade fora de combate: %s (o teto é 4%%)."},
	{ID: "wayfarer", Name: "Viajante", Branch: 2, Stat: MStatTeleport, Total: 0.30, Icon: "blink",
		Format: "Preço das Pedras de Teleporte: %s."},
	{ID: "quick_read", Name: "Leitura Rápida", Branch: 2, Stat: MStatChannel, Total: 0.40, Req: 5, Icon: "hourglass",
		Format: "Tempo para viajar (pedra e Pergaminho de Retorno): %s."},
	{ID: "way_home", Name: "Caminho de Casa", Branch: 2, Stat: MStatReturnCD, Total: 0.50, Req: 10, Icon: "swirl",
		Format: "Recarga do Pergaminho de Retorno: %s."},
}

// MasteryMaxPoints: pontos para a árvore inteira (o teto da Maestria).
var MasteryMaxPoints = len(MasteryNodes) * MasteryNodeMax

var masteryByID = func() map[string]*MasteryNode {
	m := make(map[string]*MasteryNode, len(MasteryNodes))
	for _, n := range MasteryNodes {
		m[n.ID] = n
	}
	return m
}()

// MasteryNodeByID devolve o nó (nil = não existe).
func MasteryNodeByID(id string) *MasteryNode { return masteryByID[id] }

// Value: o efeito do nó com rank pontos (soma das parcelas de MasteryRankShare).
func (n *MasteryNode) Value(rank int) float64 {
	v := 0.0
	for i := 0; i < min(rank, MasteryNodeMax); i++ {
		v += MasteryRankShare[i]
	}
	return n.Total * v
}

// Reduces: o efeito é um desconto ("−30%"), não um acréscimo.
func (n *MasteryNode) Reduces() bool {
	switch n.Stat {
	case MStatCCResist, MStatDeathLoss, MStatTeleport, MStatChannel, MStatReturnCD:
		return true
	}
	return false
}

// Text: o efeito com rank pontos, em português ("As poções curam +8%.").
func (n *MasteryNode) Text(rank int) string {
	v := n.Value(rank) * 100
	s := pctText(v)
	switch {
	case n.Stat == MStatMaterial:
		s += "%"
	case n.Reduces():
		s = "−" + s + "%"
	default:
		s = "+" + s + "%"
	}
	return fmt.Sprintf(n.Format, s)
}

// pctText: 8 → "8"; 1,5 → "1,5"; 0,6 → "0,6".
func pctText(v float64) string {
	v = math.Round(v*10) / 10
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	s := fmt.Sprintf("%.1f", v)
	return s[:len(s)-2] + "," + s[len(s)-1:]
}

// MasteryBonus: a soma dos efeitos dos nós (calculada quando a árvore muda, nunca por tick).
type MasteryBonus [mstatCount]float64

// Of: o valor de um efeito (0 = nenhum).
func (b *MasteryBonus) Of(s MasteryStat) float64 { return b[s] }

// MasteryState é o que fica salvo no personagem (characters.mastery).
type MasteryState struct {
	Points    int            `json:"points,omitempty"`     // pontos ganhos no total (o nível de Maestria)
	XP        int64          `json:"xp,omitempty"`         // XP na barra atual (0 a MasteryPointXP)
	Rested    int64          `json:"rested,omitempty"`     // bônus diário guardado: XP de barra que ainda sai 4×
	Day       int            `json:"day,omitempty"`        // último dia (Brasília) que creditou o bônus (0 = nunca)
	RespecDay int            `json:"respec_day,omitempty"` // dia do último Redistribuir
	Ranks     map[string]int `json:"ranks,omitempty"`      // nó → pontos
}

// MasteryDay: o número do dia de t no fuso loc (dias desde 1970-01-01). Troca à meia-noite.
func MasteryDay(t time.Time, loc *time.Location) int {
	y, m, d := t.In(loc).Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

// Spent: pontos gastos na árvore.
func (m *MasteryState) Spent() int {
	n := 0
	for _, r := range m.Ranks {
		n += r
	}
	return n
}

// Free: pontos para gastar.
func (m *MasteryState) Free() int { return max(0, m.Points-m.Spent()) }

// BranchPoints: pontos gastos num ramo.
func (m *MasteryState) BranchPoints(branch int) int {
	n := 0
	for id, r := range m.Ranks {
		if def := masteryByID[id]; def != nil && def.Branch == branch {
			n += r
		}
	}
	return n
}

// RestedCap: o máximo de bônus diário guardado.
func RestedCap() int64 { return MasteryPointXP * MasteryRestedDays }

// Refresh credita o bônus diário dos dias que passaram até day (1 barra por dia, até
// MasteryRestedDays). A primeira vez (quem acabou de chegar ao 40) já vem com o de hoje.
// Devolve se mudou algo.
func (m *MasteryState) Refresh(day int) bool {
	switch {
	case m.Day == 0:
		m.Day, m.Rested = day, MasteryPointXP
		return true
	case day > m.Day:
		m.Rested = min(RestedCap(), m.Rested+int64(day-m.Day)*MasteryPointXP)
		m.Day = day
		return true
	}
	return false
}

// Gain soma XP à barra (com o bônus diário, enquanto houver) e devolve a XP que entrou na
// barra e quantos pontos foram ganhos (vários de uma vez, se a XP for grande). No teto da
// Maestria a barra não enche mais.
func (m *MasteryState) Gain(xp int64) (credited int64, points int) {
	if xp <= 0 || m.Points >= MasteryMaxPoints {
		return 0, 0
	}
	credited = xp
	if m.Rested > 0 {
		// A parte da XP coberta pelo bônus conta 4×, e o bônus gasta o que encheu.
		raw := min(xp, (m.Rested+MasteryRestedMult-1)/MasteryRestedMult)
		boosted := min(raw*MasteryRestedMult, m.Rested)
		m.Rested -= boosted
		credited = boosted + (xp - raw)
	}
	m.XP += credited
	for m.XP >= MasteryPointXP && m.Points < MasteryMaxPoints {
		m.XP -= MasteryPointXP
		m.Points++
		points++
	}
	if m.Points >= MasteryMaxPoints {
		m.XP = 0
	}
	return credited, points
}

// DeathLoss: quanto da barra sai numa morte (a mesma fração de um nível do 30 em diante,
// DeathLossShare, menos o desconto da Segunda Chance), sem tirar ponto.
func (m *MasteryState) DeathLoss(b *MasteryBonus) int64 {
	return int64(math.Round(float64(MasteryPointXP) * DeathLossShare(MasteryLevel) * (1 - b.Of(MStatDeathLoss))))
}

// Usable: a árvore pode ser usada (gastar e redistribuir) no nível máximo ou por
// quem já tem pontos (ganhos quando o máximo era 30).
func (m *MasteryState) Usable(level int) bool { return level >= MasteryLevel || m.Points > 0 }

// Lose tira XP da barra (nunca abaixo de 0). Devolve quanto saiu.
func (m *MasteryState) Lose(xp int64) int64 {
	lost := min(max(0, xp), m.XP)
	m.XP -= lost
	return lost
}

// Spend gasta 1 ponto no nó. Devolve "" ou o motivo da recusa.
func (m *MasteryState) Spend(id string) string {
	def := masteryByID[id]
	switch {
	case def == nil:
		return "Esse nó de Maestria não existe."
	case m.Ranks[id] >= MasteryNodeMax:
		return def.Name + " já está no máximo."
	case m.Free() <= 0:
		return "Sem pontos de Maestria para gastar."
	case m.BranchPoints(def.Branch)-m.Ranks[id] < def.Req:
		return fmt.Sprintf("%s pede %d pontos em %s.", def.Name, def.Req, MasteryBranches[def.Branch].Name)
	}
	if m.Ranks == nil {
		m.Ranks = map[string]int{}
	}
	m.Ranks[id]++
	return ""
}

// CanRespec: o Redistribuir de hoje ainda está livre.
func (m *MasteryState) CanRespec(day int) bool { return m.RespecDay != day }

// Respec devolve todos os pontos gastos (grátis, 1 vez por dia). Devolve "" ou o motivo.
func (m *MasteryState) Respec(day int) string {
	switch {
	case m.Spent() == 0:
		return "Nenhum ponto de Maestria gasto para redistribuir."
	case !m.CanRespec(day):
		return "Você já redistribuiu a Maestria hoje. Amanhã (meia-noite de Brasília) libera de novo."
	}
	m.Ranks = nil
	m.RespecDay = day
	return ""
}

// Bonus soma os efeitos dos nós.
func (m *MasteryState) Bonus() MasteryBonus {
	var b MasteryBonus
	for id, r := range m.Ranks {
		if def := masteryByID[id]; def != nil {
			b[def.Stat] += def.Value(r)
		}
	}
	b[MStatCalmSpeed] = min(b[MStatCalmSpeed], MasterySpeedCap)
	return b
}

// NormalMastery corrige um estado salvo: nós que não existem mais saem, ranks ficam entre 1
// e MasteryNodeMax, a barra e o bônus ficam nos limites e, se a árvore pedir mais pontos
// do que o personagem tem (ou um pré-requisito quebrou), os pontos voltam todos.
func NormalMastery(m MasteryState) MasteryState {
	m.Points = min(max(0, m.Points), MasteryMaxPoints)
	m.XP = min(max(0, m.XP), MasteryPointXP-1)
	if m.Points >= MasteryMaxPoints {
		m.XP = 0
	}
	m.Rested = min(max(0, m.Rested), RestedCap())
	m.Day, m.RespecDay = max(0, m.Day), max(0, m.RespecDay)
	if len(m.Ranks) == 0 {
		m.Ranks = nil
		return m
	}
	ranks := make(map[string]int, len(m.Ranks))
	for id, r := range m.Ranks {
		if masteryByID[id] != nil && r > 0 {
			ranks[id] = min(r, MasteryNodeMax)
		}
	}
	m.Ranks = ranks
	ok := m.Spent() <= m.Points
	for id := range ranks {
		def := masteryByID[id]
		// O pré-requisito conta os pontos dos outros nós do ramo (os do próprio nó só
		// entram depois de aberto).
		if m.BranchPoints(def.Branch)-ranks[id] < def.Req {
			ok = false
		}
	}
	if !ok || len(ranks) == 0 {
		m.Ranks = nil
	}
	return m
}

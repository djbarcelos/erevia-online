package game

import (
	"fmt"
	"sort"
)

// Arquétipos de IA de monstro (roadmap 4.4, tunnel#129). Cada monstro tem um papel na luta,
// que dá para reconhecer pela silhueta e pelo ícone no quadro do alvo: o bruto persegue e
// bate (o de sempre, e o padrão de todo monstro sem Archetype), o atirador fica longe, o
// curandeiro cura, o invocador chama lacaios, o emboscador espera escondido, a matilha vem
// junta, o escudo frontal segura golpes de frente, o que divide vira dois e o explosivo
// explode. O comportamento fica em zone/archetypes.go; aqui, os dados.
//
// Neutral e NeutralByDay (MonsterDef) são à parte do papel: o neutro só ataca quem bateu
// nele, e o NeutralByDay é neutro de dia e agressivo à noite.

// Archetype é o papel do monstro na luta.
type Archetype uint8

const (
	ArchBrute    Archetype = iota // persegue e bate (o de sempre)
	ArchArcher                    // mantém distância e recua quando o alvo encosta
	ArchHealer                    // cura um aliado ferido
	ArchSummoner                  // chama lacaios durante a luta
	ArchAmbusher                  // invisível até o jogador chegar perto; o 1º golpe dói mais
	ArchPack                      // puxar um puxa o grupo de spawn inteiro
	ArchShield                    // toma menos dano de frente e vira devagar
	ArchSplitter                  // ao morrer, vira dois menores
	ArchExploder                  // explode ao chegar perto ou ao morrer, com aviso no chão
	archCount
)

// String: o nome do papel (o quadro do alvo mostra a dica no cliente).
func (a Archetype) String() string {
	switch a {
	case ArchArcher:
		return "atirador"
	case ArchHealer:
		return "curandeiro"
	case ArchSummoner:
		return "invocador"
	case ArchAmbusher:
		return "emboscador"
	case ArchPack:
		return "matilha"
	case ArchShield:
		return "escudo frontal"
	case ArchSplitter:
		return "dividir"
	case ArchExploder:
		return "explosivo"
	}
	return "bruto"
}

// ArchParams são os números do arquétipo. Zero = o padrão (FillArchDefaults põe os padrões
// de cada papel na carga dos dados); só os do papel do monstro valem.
type ArchParams struct {
	// Atirador: fica entre KeepMin e KeepMax do alvo (o alcance do ataque, Range, deve ser o
	// KeepMax) e, quando o alvo chega a menos de KeepMin, recua Retreat metros, no máximo
	// uma vez a cada RetreatCD segundos.
	KeepMin, KeepMax   float64
	Retreat, RetreatCD float64
	// Curandeiro: cura HealFrac da vida máxima do aliado mais ferido abaixo de HealBelow, a
	// até HealRange metros, a cada HealCD segundos.
	HealBelow, HealFrac, HealCD, HealRange float64
	// Invocador: chama SummonCount lacaios (Summon, um id de Monsters) a cada SummonCD
	// segundos de luta (o primeiro SummonFirst segundos depois de a luta começar), com no
	// máximo SummonMax vivos. Os lacaios morrem com ele.
	Summon                 string
	SummonCount, SummonMax int
	SummonCD, SummonFirst  float64
	// Emboscador: invisível até um jogador chegar a AmbushRange metros; o primeiro golpe
	// depois de aparecer causa AmbushMult × o dano.
	AmbushRange, AmbushMult float64
	// Matilha: o último vivo do grupo entra em frenesi (+PackFrenzy de velocidade).
	PackFrenzy float64
	// Escudo frontal: golpes físicos que vêm de um cone de ShieldArc graus à frente causam
	// ShieldCut a menos; o monstro vira para o alvo a TurnRate graus por segundo.
	ShieldArc, ShieldCut, TurnRate float64
	// Dividir: ao morrer, vira SplitCount SplitInto (um id de Monsters), 2 níveis abaixo.
	SplitInto  string
	SplitCount int
	// Explosivo: com o alvo a até FuseRange metros (da borda do corpo), para e explode em
	// BlastDelay segundos, num círculo de BlastRadius, com BlastMult × o ataque. Se morrer
	// antes, explode do mesmo jeito onde caiu.
	FuseRange, BlastRadius, BlastDelay, BlastMult float64
}

// Padrões dos arquétipos (roadmap 4.4).
const (
	ArchSplitLevels = 2 // os pedaços do que divide nascem 2 níveis abaixo
)

var archDefaults = map[Archetype]ArchParams{
	ArchArcher:   {KeepMin: 7, KeepMax: 10, Retreat: 5, RetreatCD: 6},
	ArchHealer:   {HealBelow: 0.5, HealFrac: 0.15, HealCD: 8, HealRange: 12},
	ArchSummoner: {SummonCount: 2, SummonMax: 4, SummonCD: 20, SummonFirst: 2},
	ArchAmbusher: {AmbushRange: 4.5, AmbushMult: 1.5},
	ArchPack:     {PackFrenzy: 0.2},
	ArchShield:   {ShieldArc: 120, ShieldCut: 0.6, TurnRate: 90},
	ArchSplitter: {SplitCount: 2},
	ArchExploder: {FuseRange: 2, BlastRadius: 2.5, BlastDelay: 1, BlastMult: 2},
}

// FillArchDefaults põe os padrões do papel nos parâmetros que ficaram em zero.
func FillArchDefaults(m *MonsterDef) {
	d := archDefaults[m.Archetype]
	p := &m.Arch
	fill := func(v *float64, def float64) {
		if *v == 0 {
			*v = def
		}
	}
	fillInt := func(v *int, def int) {
		if *v == 0 {
			*v = def
		}
	}
	fill(&p.KeepMin, d.KeepMin)
	fill(&p.KeepMax, d.KeepMax)
	fill(&p.Retreat, d.Retreat)
	fill(&p.RetreatCD, d.RetreatCD)
	fill(&p.HealBelow, d.HealBelow)
	fill(&p.HealFrac, d.HealFrac)
	fill(&p.HealCD, d.HealCD)
	fill(&p.HealRange, d.HealRange)
	fillInt(&p.SummonCount, d.SummonCount)
	fillInt(&p.SummonMax, d.SummonMax)
	fill(&p.SummonCD, d.SummonCD)
	fill(&p.SummonFirst, d.SummonFirst)
	fill(&p.AmbushRange, d.AmbushRange)
	fill(&p.AmbushMult, d.AmbushMult)
	fill(&p.PackFrenzy, d.PackFrenzy)
	fill(&p.ShieldArc, d.ShieldArc)
	fill(&p.ShieldCut, d.ShieldCut)
	fill(&p.TurnRate, d.TurnRate)
	fillInt(&p.SplitCount, d.SplitCount)
	fill(&p.FuseRange, d.FuseRange)
	fill(&p.BlastRadius, d.BlastRadius)
	fill(&p.BlastDelay, d.BlastDelay)
	fill(&p.BlastMult, d.BlastMult)
}

// NeutralNow: o monstro está neutro agora (só ataca quem bateu nele). night = IsNight.
func (m *MonsterDef) NeutralNow(night bool) bool {
	return m.Neutral || (m.NeutralByDay && !night)
}

// HasRole: o monstro tem papel ou é neutro (vai para o cliente; os brutos comuns não).
func (m *MonsterDef) HasRole() bool {
	return m.Archetype != ArchBrute || m.Neutral || m.NeutralByDay
}

// BalanceBaseline: o monstro entra nas contas de caça (a tabela teórica, em
// progression_table.go, e o simulador, em balance/hunt.go). Os de papel e os neutros ficam de
// fora: são poucos por área, e a luta modelada (um contra um, de frente, sem fugir) não sabe
// recuar, curar, invocar, se esconder, flanquear nem sair da explosão.
func (m *MonsterDef) BalanceBaseline() bool { return !m.HasRole() }

// ValidateArchetype confere o papel do monstro (roda na carga dos dados, depois dos padrões).
func ValidateArchetype(m *MonsterDef) error {
	p := m.Arch
	bad := func(format string, args ...any) error {
		return fmt.Errorf("monstro %s (%s): %s", m.ID, m.Archetype, fmt.Sprintf(format, args...))
	}
	if m.Archetype >= archCount {
		return bad("arquétipo desconhecido")
	}
	if m.Neutral && m.NeutralByDay {
		return bad("Neutral e NeutralByDay juntos")
	}
	if (m.Neutral || m.NeutralByDay) && m.Aggressive {
		return bad("neutro não pode ser Aggressive (o NeutralByDay já é agressivo à noite)")
	}
	switch m.Archetype {
	case ArchArcher:
		if p.KeepMin <= 0 || p.KeepMax <= p.KeepMin || p.Retreat <= 0 || p.RetreatCD <= 0 {
			return bad("distâncias e recuo do atirador")
		}
		if m.Range < p.KeepMin {
			return bad("alcance %.1f m menor que a distância mínima %.1f m", m.Range, p.KeepMin)
		}
	case ArchHealer:
		if p.HealBelow <= 0 || p.HealBelow >= 1 || p.HealFrac <= 0 || p.HealCD <= 0 || p.HealRange <= 0 {
			return bad("números da cura")
		}
	case ArchSummoner:
		s := Monsters[p.Summon]
		if s == nil || s.Archetype == ArchSummoner || s.Archetype == ArchSplitter {
			return bad("lacaio %q desconhecido, ou que também invoca ou divide", p.Summon)
		}
		if p.SummonCount <= 0 || p.SummonMax < p.SummonCount || p.SummonCD <= 0 {
			return bad("quantidade e recarga dos lacaios")
		}
	case ArchAmbusher:
		if p.AmbushRange <= 0 || p.AmbushMult < 1 {
			return bad("alcance e golpe da emboscada")
		}
	case ArchShield:
		if p.ShieldArc <= 0 || p.ShieldArc > 360 || p.ShieldCut <= 0 || p.ShieldCut >= 1 || p.TurnRate <= 0 {
			return bad("cone, redução e giro do escudo")
		}
	case ArchSplitter:
		s := Monsters[p.SplitInto]
		if s == nil || s.Archetype == ArchSplitter || p.SplitCount <= 0 {
			return bad("pedaço %q desconhecido ou que divide de novo", p.SplitInto)
		}
	case ArchExploder:
		if p.FuseRange <= 0 || p.BlastRadius <= 0 || p.BlastMult <= 0 {
			return bad("pavio, raio e dano da explosão")
		}
		if p.BlastDelay < m.MinAbilityDelay() {
			return bad("aviso de %.2f s, abaixo do mínimo de %.1f s", p.BlastDelay, m.MinAbilityDelay())
		}
		if p.BlastRadius > MaxAbilityReach {
			return bad("explosão maior que o alcance máximo dos avisos")
		}
	}
	return nil
}

func init() {
	ids := make([]string, 0, len(Monsters))
	for id := range Monsters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		FillArchDefaults(Monsters[id])
	}
	for _, id := range ids {
		if err := ValidateArchetype(Monsters[id]); err != nil {
			panic(err)
		}
	}
}

package game

import (
	"fmt"
	"sort"
)

// Habilidades de monstro com aviso no chão (tunnel#130): o monstro desenha a forma, e o
// golpe cai no fim do atraso em quem ainda estiver dentro (zone/abilities.go). Tudo por
// dados, em MonsterDef.Abilities; o raid boss usa o mesmo caminho.

// AbilityShape é a forma do aviso no chão.
type AbilityShape uint8

const (
	ShapeCircle AbilityShape = iota // Radius em volta do ponto
	ShapeCone                       // Radius e abertura Arc, saindo do monstro
	ShapeLine                       // Length × Width, saindo do monstro
	ShapeDonut                      // anel entre Inner e Radius (o buraco é seguro)
)

// AbilityTarget diz onde o aviso cai. Círculo e anel ficam centrados no ponto; cone e
// linha sempre saem do monstro, apontados para o ponto.
type AbilityTarget uint8

const (
	AimSelf    AbilityTarget = iota // o próprio monstro (cone e linha: para quem ele ataca)
	AimCurrent                      // quem o monstro está atacando
	AimRandom                       // Count jogadores sorteados entre os que estão na luta
	// Marca: um jogador da luta (de preferência não o tanque); o aviso segue o marcado até
	// disparar e acerta quem estiver perto dele, nunca o próprio marcado.
	AimMark
)

// AbilityLook é o visual do aviso no cliente (TelegraphKind no protocolo).
type AbilityLook uint8

const (
	LookStrike AbilityLook = iota // pancada de monstro comum ou chefe de campo
	LookWave                      // Onda de Ossos (raid)
	LookRain                      // Chuva de Ossos (raid)
	LookMark                      // Marca da Morte (raid)
)

// Ids dos golpes avisados nos CombatEvent: o cliente mostra o número sem animar o monstro
// batendo (o visual sai do aviso). Os do raid ficam com os números de antes.
const (
	HitRaidWave uint32 = 9001 + iota
	HitRaidRain
	HitRaidMark
	HitMonsterAbility // padrão das habilidades sem HitID
)

// Atraso mínimo entre o aviso e o golpe (roadmap, T4): ping de 150 ms + reação de 250 ms
// + sair de 3 m a 5 m/s em comuns; elites e chefes têm formas maiores.
const (
	MinDelayCommon = 1.0
	MinDelayTough  = 1.5
	// MaxAbilityReach: maior distância da origem até a borda da forma. O aviso só vai para
	// quem conhece o monstro ou o marcado (a área de interesse é de 70 m).
	MaxAbilityReach = 30.0
)

// AbilityDef é uma habilidade avisada de um monstro.
type AbilityDef struct {
	ID     string // único no monstro
	Look   AbilityLook
	Shape  AbilityShape
	Target AbilityTarget
	Count  int // AimRandom: quantos jogadores (0 = 1)

	Radius float64 // círculo, cone e anel: alcance a partir da origem (m)
	Inner  float64 // anel: raio do buraco seguro (m)
	Arc    float64 // cone: abertura total (graus)
	Length float64 // linha (m)
	Width  float64 // linha (m)

	Delay    float64 // segundos entre o aviso e o golpe
	Cooldown float64 // segundos entre dois usos
	// First: segundos até o primeiro uso, contados do começo da luta (ou da chegada à
	// MinPhase, nas habilidades de fase).
	First    float64
	Mult     float64    // multiplicador do ataque do monstro
	Effect   *EffectDef // em quem for atingido (nil = só o dano)
	OnMarked *EffectDef // AimMark: no marcado, na hora de marcar
	Warn     string     // AimMark: aviso no chat para o marcado
	MinPhase int        // fase mínima (raid: 1–3; os outros monstros estão sempre na 1)
	HitID    uint32     // id do golpe no CombatEvent (0 = HitMonsterAbility)
}

// Tough: elites e chefes (o aviso precisa de mais tempo).
func (m *MonsterDef) Tough() bool { return m.Boss || m.Raid || m.Elite }

// MinAbilityDelay é o atraso mínimo das habilidades do monstro.
func (m *MonsterDef) MinAbilityDelay() float64 {
	if m.Tough() {
		return MinDelayTough
	}
	return MinDelayCommon
}

// Hit: o id do golpe no CombatEvent.
func (a *AbilityDef) Hit() uint32 {
	if a.HitID != 0 {
		return a.HitID
	}
	return HitMonsterAbility
}

// Reach: da origem até a borda mais distante da forma.
func (a *AbilityDef) Reach() float64 {
	if a.Shape == ShapeLine {
		return a.Length + a.Width/2
	}
	return a.Radius
}

// ValidateAbilities confere as habilidades de um monstro (roda na carga dos dados).
func ValidateAbilities(m *MonsterDef) error {
	seen := map[string]bool{}
	for i := range m.Abilities {
		a := &m.Abilities[i]
		bad := func(format string, args ...any) error {
			return fmt.Errorf("monstro %s, habilidade %q: %s", m.ID, a.ID, fmt.Sprintf(format, args...))
		}
		if a.ID == "" || seen[a.ID] {
			return bad("id vazio ou repetido")
		}
		seen[a.ID] = true
		if floor := m.MinAbilityDelay(); a.Delay < floor {
			return bad("atraso de %.2f s, abaixo do mínimo de %.1f s", a.Delay, floor)
		}
		if a.Cooldown < a.Delay {
			return bad("recarga de %.2f s menor que o atraso", a.Cooldown)
		}
		if a.Mult < 0 || a.First < 0 || a.MinPhase < 0 || a.Count < 0 {
			return bad("valor negativo")
		}
		switch a.Shape {
		case ShapeCircle:
			if a.Radius <= 0 {
				return bad("círculo sem raio")
			}
		case ShapeCone:
			if a.Radius <= 0 || a.Arc <= 0 || a.Arc > 360 {
				return bad("cone precisa de raio e abertura entre 0 e 360°")
			}
		case ShapeLine:
			if a.Length <= 0 || a.Width <= 0 {
				return bad("linha precisa de comprimento e largura")
			}
		case ShapeDonut:
			if a.Inner <= 0 || a.Radius <= a.Inner {
				return bad("anel precisa de 0 < raio interno < raio")
			}
		default:
			return bad("forma desconhecida")
		}
		if a.Target == AimMark && a.Shape != ShapeCircle && a.Shape != ShapeDonut {
			return bad("a marca segue o marcado: só círculo ou anel")
		}
		if a.Target > AimMark {
			return bad("alvo desconhecido")
		}
		if a.Reach() > MaxAbilityReach {
			return bad("alcance de %.1f m passa do máximo de %.0f m", a.Reach(), MaxAbilityReach)
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
		if err := ValidateAbilities(Monsters[id]); err != nil {
			panic(err)
		}
	}
}

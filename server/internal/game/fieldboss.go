package game

import (
	"fmt"
	"sort"
)

// Chefe de campo (tunnel#132): o chefe de uma área, com o que antes só o raid tinha, mas em
// minutos e para um grupo. Renasce num mínimo mais uma janela (relógio de parede, salvo no
// banco), tem fases por vida, com as habilidades avisadas de MonsterDef.Abilities (MinPhase),
// divide a XP e o saque pelo dano e deixa um túmulo no chão (zone/fieldboss.go).

// FieldBossDef são os dados do chefe de campo (MonsterDef.FieldBoss).
type FieldBossDef struct {
	// Renascimento em minutos de relógio: sorteado entre os dois a partir da morte. O mapa
	// mostra a janela inteira, nunca o minuto.
	RespawnMin, RespawnMax float64
	// Place completa os anúncios ("desperta na lagoa de Vau Velho").
	Place string
	// Instance: o chefe de uma masmorra (catacumbas.go). Nasce com a cópia e não renasce: sem
	// janela, sem renascimento salvo no banco e sem túmulo. RespawnMin e RespawnMax ficam em zero.
	Instance bool
	// Phases: a fase 2 é Phases[0], a 3 é Phases[1]... A 1 é a luta inteira antes delas.
	Phases []FieldPhase
}

// FieldPhase começa quando a vida do chefe cai a AtHP (fração da vida máxima).
type FieldPhase struct {
	AtHP float64
	Say  string // fala do chefe (aviso do sistema para quem está perto)
	// Invocação ao entrar na fase. Se ReturnAfter > 0, quem sobreviver a esse tempo volta
	// para o chefe e o cura em ReturnHeal da vida máxima (proporcional aos que voltam).
	Summon      string
	SummonCount int
	ReturnAfter float64
	ReturnHeal  float64
	Enrage      bool // ataques 40% mais rápidos a partir daqui
}

// FieldBossEnrageSpeed: a fúria de fase deixa o intervalo entre golpes nesta fração.
const FieldBossEnrageSpeed = 1.4

// PhaseCount: quantas fases o chefe tem.
func (f *FieldBossDef) PhaseCount() int { return len(f.Phases) + 1 }

// Marks: as vidas (fração) em que as fases começam, para as marcas da barra do cliente.
func (f *FieldBossDef) Marks() []float32 {
	out := make([]float32, len(f.Phases))
	for i, p := range f.Phases {
		out[i] = float32(p.AtHP)
	}
	return out
}

// ValidateFieldBoss confere o chefe de campo de um monstro (roda na carga dos dados).
func ValidateFieldBoss(m *MonsterDef) error {
	f := m.FieldBoss
	if f == nil {
		return nil
	}
	bad := func(format string, args ...any) error {
		return fmt.Errorf("chefe de campo %s: %s", m.ID, fmt.Sprintf(format, args...))
	}
	if !m.Boss || m.Raid {
		return bad("precisa de Boss e não pode ser Raid")
	}
	if f.Instance {
		if f.RespawnMin != 0 || f.RespawnMax != 0 {
			return bad("o chefe de masmorra não renasce")
		}
	} else if f.RespawnMin <= 0 || f.RespawnMax < f.RespawnMin {
		return bad("renascimento de %.0f a %.0f min inválido", f.RespawnMin, f.RespawnMax)
	}
	last := 1.0
	for i, p := range f.Phases {
		if p.AtHP <= 0 || p.AtHP >= last {
			return bad("fase %d começa em %.2f da vida: precisa ser menor que a anterior e maior que 0", i+2, p.AtHP)
		}
		last = p.AtHP
		if p.Summon != "" {
			if Monsters[p.Summon] == nil || p.SummonCount <= 0 {
				return bad("fase %d invoca %q x%d", i+2, p.Summon, p.SummonCount)
			}
			if p.ReturnAfter < 0 || p.ReturnHeal < 0 || p.ReturnHeal > 1 || (p.ReturnAfter > 0) != (p.ReturnHeal > 0) {
				return bad("fase %d: a volta das invocações precisa de tempo e cura juntos (cura de 0 a 1)", i+2)
			}
		}
	}
	for _, a := range m.Abilities {
		if a.MinPhase > f.PhaseCount() {
			return bad("habilidade %q pede a fase %d, e o chefe tem %d", a.ID, a.MinPhase, f.PhaseCount())
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
		if err := ValidateFieldBoss(Monsters[id]); err != nil {
			panic(err)
		}
	}
}

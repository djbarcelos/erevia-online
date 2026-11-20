package game

import (
	"fmt"
	"slices"
)

// Evoluções de classe (docs/evolucoes). No nível 20 cada classe escolhe uma de duas
// evoluções (as especializações de antes, mesmo id). Ao evoluir a árvore base some: o
// personagem passa a ter só a árvore da evolução (3 ramos, marcos 20/25/30/35/40) e os
// pontos vão até o teto (1 por nível). As skills do degrau 20 de antes (49–60) e as de
// especialização (61–84) mantêm o id e viram nós das evoluções.

// NodeKind é o tipo de nó na árvore da evolução.
type NodeKind uint8

const (
	NodeActive   NodeKind = iota // ativa (até 3 ranks)
	NodePassive                  // passiva (até 5 ranks)
	NodeModifier                 // altera uma ativa (Modifies); exige rank 1 nela
	NodeKey                      // chave do ramo (marco 40, 1 rank)
	NodeSupreme                  // suprema (marco 60, chega com o teto 60)
)

// SkillReq: rank mínimo numa skill para aprender esta.
type SkillReq struct {
	ID   uint32
	Rank int
}

// SkillMod: o que um modificador soma, por rank dele, na skill que modifica.
type SkillMod struct {
	MultPct   Scale  // +fração do multiplicador de dano (0,1 = +10%)
	CD, MP    Scale  // somados à recarga e ao custo (negativo = menos)
	Splash    Scale  // +raio em volta do alvo
	ExtraCrit Scale  // +chance de crítico
	Hits      [4]int // golpes extras por rank (índice = rank; 0 = nenhum)
	NoMiss    bool   // passa a não errar
	EffectDur Scale  // +segundos no efeito no alvo (ou em si, se não houver no alvo)
	// EffectValue soma ao valor do efeito principal; EffectValPct multiplica (1 + v).
	EffectValue, EffectValPct Scale
	Range                     Scale // +alcance (ou raio, nas de área em volta de si)
	// Área extra: alvos, dano e sem efeito (Eco do Escudo).
	SplashMax      int
	SplashMult     Scale
	SplashNoEffect bool
	ExtraOnTarget  []*EffectSpec
	// Dano a mais contra quem tem algum dos efeitos (Veredito).
	BonusVs    []EffectKind
	BonusVsPct Scale
	ResOnHit   Scale // recurso a mais por alvo atingido
}

// EvolutionMilestones: os marcos da árvore evoluída até o teto 40 (45–60 com o teto 60).
var EvolutionMilestones = []int{20, 25, 30, 35, 40}

// MilestoneBranchPoints: pontos no ramo exigidos em cada marco (README, seção 2).
func MilestoneBranchPoints(level int) int {
	switch {
	case level >= 40:
		return 15
	case level >= 35:
		return 10
	case level >= 30:
		return 5
	}
	return 0
}

// EvolutionChangeCost: trocar de evolução depois de escolher (2.500 × nível).
func EvolutionChangeCost(level int) int64 { return 2500 * int64(level) }

// RespecCost: redistribuir a árvore da evolução (500 × nível).
func RespecCost(level int) int64 { return 500 * int64(level) }

// StarterItemsFlag: marca (em quests.done) de que a evolução já entregou os itens iniciais.
func StarterItemsFlag(spec string) string { return "evo_itens:" + spec }

// place: lugar de uma skill que já existia na árvore de uma evolução.
type place struct {
	spec   string
	branch int
	level  int
}

// evoPlaces: as skills do degrau 20 e de especialização, por evolução (docs/evolucoes).
var evoPlaces = map[uint32]place{
	// Bastião: Muralha · Protetor · Provocador
	SkillBastionVigor: {"bastion", 0, 20}, SkillBastionLast: {"bastion", 0, 35},
	SkillAegis: {"bastion", 1, 20}, SkillBastionDefy: {"bastion", 2, 20}, SkillBulwark: {"bastion", 2, 20},
	// Paladino: Julgamento · Bênçãos · Expiação
	SkillHeraldDawn: {"herald", 0, 20}, SkillHorizon: {"herald", 0, 20},
	SkillHeraldBless: {"herald", 1, 20}, SkillHeraldSun: {"herald", 2, 20},
	// Berserker: Carnificina · Frenesi · Execução
	SkillSeismicAxe: {"bloodrager", 0, 20}, SkillBloodWhirl: {"bloodrager", 1, 20},
	SkillRagerRed: {"bloodrager", 2, 20}, SkillRagerBlind: {"bloodrager", 2, 20}, SkillRagerCarnage: {"bloodrager", 2, 35},
	// Senhor da Guerra: Comando · Investida · Domínio
	SkillWarlordCry: {"warlord", 0, 20}, SkillWarlordBanner: {"warlord", 0, 20},
	SkillWarlordCharge: {"warlord", 1, 20}, SkillStoneSkin: {"warlord", 2, 20},
	// Elementalista: Chamas · Geada · Convergência
	SkillElemLance: {"elementalist", 0, 20}, SkillInferno: {"elementalist", 0, 20},
	SkillAbsoluteZero: {"elementalist", 1, 20}, SkillElemFocus: {"elementalist", 2, 20}, SkillElemCata: {"elementalist", 2, 30},
	// Arcanista: Maldições · Pragas · Arcano
	SkillOccDrain: {"occultist", 0, 20}, SkillOccAbyss: {"occultist", 0, 20},
	SkillOccRot: {"occultist", 1, 20}, SkillArcaneBlast: {"occultist", 2, 20},
	// Assassino: Lâminas · Sombras · Execução
	SkillCutBlades: {"cutthroat", 0, 20}, SkillFatalBlow: {"cutthroat", 0, 20}, SkillCutSentence: {"cutthroat", 0, 30},
	SkillShadowStrike: {"cutthroat", 1, 20}, SkillCutInstinct: {"cutthroat", 2, 20},
	// Patrulheiro: Arqueria · Armadilhas · Caçada
	SkillHuntRain: {"hunter", 0, 30}, SkillHuntNet: {"hunter", 1, 20}, SkillMiasma: {"hunter", 1, 20},
	SkillHuntKnife: {"hunter", 2, 20},
}

// Ids das skills novas das evoluções (docs/evolucoes, a partir do 85; provisórios até a
// fase de conteúdo de cada classe). Valores explícitos: cada evolução tem a sua faixa.
const (
	SkillBastionGuard   uint32 = 85  // Bastião: Guarda Inabalável
	SkillBastionBash    uint32 = 95  // Bastião: Golpe de Escudo (evol.)
	SkillHeraldStrike   uint32 = 100 // Paladino: Golpe Radiante
	SkillHeraldLight    uint32 = 106 // Paladino: Bênção da Luz
	SkillRagerBrutal    uint32 = 117 // Berserker: Golpe Brutal (evol.)
	SkillRagerWrath     uint32 = 123 // Berserker: Cólera (evol. da Fúria)
	SkillWarlordCommand uint32 = 138 // Senhor da Guerra: Golpe de Comando
	SkillElemFireball   uint32 = 150 // Elementalista: Bola de Fogo (evol.)
	SkillElemNova       uint32 = 156 // Elementalista: Nova de Gelo (evol.)
	SkillCutBackstab    uint32 = 185 // Assassino: Golpe Furtivo (evol.)
	SkillCutStep        uint32 = 191 // Assassino: Passo Sombrio (evol.)
	SkillHuntShot       uint32 = 203 // Patrulheiro: Tiro Preciso
)

// evolved copia uma skill da base com outro id, na evolução (mesmo efeito e números).
func evolved(baseID, id uint32, spec string, branch int) *SkillDef {
	b := baseSkill(baseID)
	s := *b
	s.ID, s.Spec, s.Branch, s.Level, s.Tier, s.EvolvedFrom = id, spec, branch, 20, 0, baseID
	return &s
}

// baseSkill: a definição da base (antes do init terminar, procura nas listas das classes).
func baseSkill(id uint32) *SkillDef {
	for _, list := range [][]*SkillDef{knightTree(), barbarianTree(), mageTree(), rogueTree()} {
		for _, s := range list {
			if s.ID == id {
				return s
			}
		}
	}
	panic(fmt.Sprintf("skill base %d não existe", id))
}

// evoSkills: as skills novas (por enquanto as 2 iniciais de cada evolução; o resto entra na
// fase de conteúdo de cada classe).
func evoSkills() []*SkillDef {
	k, b, r := "knight", "barbarian", "rogue"
	list := []*SkillDef{
		{ID: SkillBastionGuard, Class: k, Spec: "bastion", Branch: 0, Level: 20, MaxRank: 3, Name: "Guarda Inabalável",
			Target: TargetSelf, MP: Scale{15, 0}, CD: Scale{20, 0},
			OnSelf: &EffectSpec{Kind: EffectBuffDef, Duration: Scale{8, 0}, Value: Scale{0.4, 0.1}},
			FX:     "fortress", Description: "Planta o escudo no chão e não sai do lugar.",
			Text: func(r int) string { return fmt.Sprintf("+%s de defesa por 8 s.", pct(Scale{0.4, 0.1}.At(r))) }},
		evolved(SkillShieldBash, SkillBastionBash, "bastion", 2),
		{ID: SkillHeraldStrike, Class: k, Spec: "herald", Branch: 0, Level: 20, MaxRank: 3, Name: "Golpe Radiante",
			Target: TargetEnemy, Range: 2.5, MP: Scale{12, 0}, CD: Scale{6, 0}, Mult: Scale{1.7, 0.2},
			FX: "k_judg", Description: "A lâmina brilha e corta com a luz da ordem.",
			Text: func(r int) string { return fmt.Sprintf("Causa %s do ataque.", pct(Scale{1.7, 0.2}.At(r))) }},
		{ID: SkillHeraldLight, Class: k, Spec: "herald", Branch: 1, Level: 20, MaxRank: 3, Name: "Bênção da Luz",
			Target: TargetSelf, MP: Scale{20, 0}, CD: Scale{30, 0}, AllyRadius: 12,
			OnSelf: &EffectSpec{Kind: EffectBuffAtk, Duration: Scale{15, 0}, Value: Scale{0.1, 0.05}},
			FX:     "k_cry", Description: "Uma prece curta que acende o grupo.",
			Text: func(r int) string {
				return fmt.Sprintf("Você e os aliados a até 12 m ganham +%s de ataque por 15 s.", pct(Scale{0.1, 0.05}.At(r)))
			}},
		evolved(SkillBrutalStrike, SkillRagerBrutal, "bloodrager", 0),
		evolved(SkillFury, SkillRagerWrath, "bloodrager", 1),
		{ID: SkillWarlordCommand, Class: b, Spec: "warlord", Branch: 1, Level: 20, MaxRank: 3, Name: "Golpe de Comando",
			Target: TargetEnemy, Range: 2.5, MP: Scale{10, 0}, CD: Scale{6, 0}, Mult: Scale{2.0, 0.25},
			FX: "brutal_strike", Description: "Abre a luta e dá o ritmo para a tropa.",
			Text: func(r int) string { return fmt.Sprintf("Causa %s do ataque.", pct(Scale{2.0, 0.25}.At(r))) }},
		evolved(SkillFireball, SkillElemFireball, "elementalist", 0),
		evolved(SkillIceNova, SkillElemNova, "elementalist", 1),
		evolved(SkillBackstab, SkillCutBackstab, "cutthroat", 0),
		evolved(SkillShadowStep, SkillCutStep, "cutthroat", 1),
		{ID: SkillHuntShot, Class: r, Spec: "hunter", Branch: 0, Level: 20, MaxRank: 3, Name: "Tiro Preciso",
			Target: TargetEnemy, Range: 14, MP: Scale{12, 0}, CD: Scale{5, 0}, Mult: Scale{1.8, 0.25},
			FX: "r_mark", Description: "Respira fundo e acerta onde mirou.",
			Text: func(r int) string {
				return fmt.Sprintf("Causa %s do ataque à distância (até 14 m).", pct(Scale{1.8, 0.25}.At(r)))
			}},
	}
	for _, s := range list {
		if s.ID == SkillRagerWrath {
			s.Name = "Cólera" // a Fúria vira recurso do Berserker; a skill muda de nome
		}
	}
	return list
}

// placeEvolutionSkills põe as skills antigas (49–84) na árvore das evoluções e ajusta o
// degrau (marco) e os pontos no ramo exigidos de cada nó evoluído.
func placeEvolutionSkills() {
	for id, pl := range evoPlaces {
		s := Skills[id]
		s.Spec, s.Branch, s.Level = pl.spec, pl.branch, pl.level
	}
	for _, s := range Skills {
		if s.Spec == "" {
			continue
		}
		s.Tier = (s.Level - EvolutionMilestones[0]) / 5
		if s.BranchPoints == 0 {
			s.BranchPoints = MilestoneBranchPoints(s.Level)
		}
		if s.Node == NodeActive && s.IsPassive() {
			s.Node = NodePassive
		}
	}
}

// EvolutionTree devolve as skills da evolução, por ramo e marco.
func EvolutionTree(spec string) []*SkillDef {
	var out []*SkillDef
	for _, s := range Skills {
		if s.Spec == spec && spec != "" {
			out = append(out, s)
		}
	}
	sortSkills(out)
	return out
}

// BranchRanks: pontos gastos em cada ramo da árvore (da evolução ou da base).
func BranchRanks(spec string, ranks map[uint32]int) [3]int {
	var out [3]int
	for id, r := range ranks {
		if s := Skills[id]; s != nil && s.Spec == spec && s.Branch >= 0 && s.Branch < 3 {
			out[s.Branch] += r
		}
	}
	return out
}

// EvolvedHotbarID: a skill que ocupa a tecla de uma skill da base depois de evoluir (as 2
// iniciais da classe viram as 2 iniciais da evolução; 0 = a tecla fica vazia).
func EvolvedHotbarID(class, spec string, id uint32) uint32 {
	d := SpecFor(class, spec)
	if d == nil {
		return 0
	}
	for i, s := range ClassSkills(class) {
		if s.ID == id && i < len(d.StarterSkills) {
			return d.StarterSkills[i]
		}
	}
	for _, s := range EvolutionTree(spec) {
		if s.EvolvedFrom == id {
			return s.ID
		}
	}
	return 0
}

// EffectiveSkill devolve a skill com os modificadores aprendidos aplicados (a própria
// skill se nenhum modificador dela foi aprendido).
func EffectiveSkill(s *SkillDef, ranks map[uint32]int) *SkillDef {
	if s == nil {
		return nil
	}
	var mods []*SkillDef
	for _, m := range modifiersOf[s.ID] {
		if ranks[m.ID] > 0 && m.Mod != nil {
			mods = append(mods, m)
		}
	}
	if len(mods) == 0 {
		return s
	}
	out := *s
	for _, m := range mods {
		r := ranks[m.ID]
		mod := m.Mod
		out.Mult = scaleTimes(out.Mult, 1+mod.MultPct.atOrZero(r))
		out.CD = scalePlus(out.CD, mod.CD.atOrZero(r))
		out.MP = scalePlus(out.MP, mod.MP.atOrZero(r))
		out.Splash += mod.Splash.atOrZero(r)
		out.ExtraCrit += mod.ExtraCrit.atOrZero(r)
		if r < len(mod.Hits) {
			out.Hits += mod.Hits[r]
		}
		out.NoMiss = out.NoMiss || mod.NoMiss
		out.Range += mod.Range.atOrZero(r)
		if mod.SplashMax > 0 {
			out.SplashMax = mod.SplashMax
		}
		if mod.SplashMult != (Scale{}) {
			out.SplashMult = Scale{Base: mod.SplashMult.At(r)}
		}
		out.SplashNoEffect = out.SplashNoEffect || mod.SplashNoEffect
		if len(mod.ExtraOnTarget) > 0 {
			out.ExtraOnTarget = append(append([]*EffectSpec{}, out.ExtraOnTarget...), mod.ExtraOnTarget...)
		}
		if len(mod.BonusVs) > 0 {
			out.BonusVs = append(append([]EffectKind{}, out.BonusVs...), mod.BonusVs...)
			out.BonusVsPct += mod.BonusVsPct.atOrZero(r)
		}
		out.ResOnHit += mod.ResOnHit.atOrZero(r)
		dur, val, valPct := mod.EffectDur.atOrZero(r), mod.EffectValue.atOrZero(r), mod.EffectValPct.atOrZero(r)
		if dur != 0 || val != 0 || valPct != 0 {
			change := func(e *EffectSpec) *EffectSpec {
				c := *e
				c.Duration = scalePlus(c.Duration, dur)
				c.Value = scaleTimes(scalePlus(c.Value, val), 1+valPct)
				return &c
			}
			if out.OnTarget != nil {
				out.OnTarget = change(out.OnTarget)
			} else if out.OnSelf != nil {
				out.OnSelf = change(out.OnSelf)
			}
		}
	}
	return &out
}

// atOrZero: o valor no rank (0 se a escala for vazia).
func (s Scale) atOrZero(rank int) float64 {
	if s == (Scale{}) {
		return 0
	}
	return s.At(rank)
}

func scalePlus(s Scale, v float64) Scale  { return Scale{s.Base + v, s.Per} }
func scaleTimes(s Scale, f float64) Scale { return Scale{s.Base * f, s.Per * f} }

// modifiersOf: skill → modificadores dela (preenchido no init).
var modifiersOf = map[uint32][]*SkillDef{}

func indexModifiers() {
	for _, s := range Skills {
		if s.Modifies != 0 {
			modifiersOf[s.Modifies] = append(modifiersOf[s.Modifies], s)
		}
	}
	for _, list := range modifiersOf {
		slices.SortFunc(list, func(a, b *SkillDef) int { return int(a.ID) - int(b.ID) })
	}
}

// validateEvolution confere as regras da árvore evoluída numa skill aprendida: marco,
// pontos no ramo e pré-requisitos.
func validateEvolution(s *SkillDef, ranks map[uint32]int, branch [3]int) error {
	if s.BranchPoints > 0 && branch[s.Branch]-ranks[s.ID] < s.BranchPoints {
		return fmt.Errorf("%s exige %d pontos no ramo", s.Name, s.BranchPoints)
	}
	for _, req := range s.Requires {
		if ranks[req.ID] < req.Rank {
			return fmt.Errorf("%s exige %s %d", s.Name, Skills[req.ID].Name, req.Rank)
		}
	}
	if s.Modifies != 0 && ranks[s.Modifies] < 1 {
		return fmt.Errorf("%s exige %s", s.Name, Skills[s.Modifies].Name)
	}
	return nil
}

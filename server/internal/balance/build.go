// Package balance simula o combate e a subida de nível com as fórmulas reais do pacote
// game, sem servidor: dano por segundo, tempo para matar e para morrer, XP por hora e tempo
// até cada nível, por classe, caminho e nível. Tudo é média por fórmula (determinístico).
// O comando cmd/balance imprime as tabelas; budget_test.go guarda o orçamento de poder.
package balance

import (
	"fmt"
	"slices"

	"erevia/server/internal/game"
)

// ---------------------------------------------------------------------------------------
// O personagem típico
// ---------------------------------------------------------------------------------------

// Path é uma classe com o caminho que ela segue do nível 20 em diante.
type Path struct{ Class, Spec string }

// Name: "Cavaleiro" (antes do 20) ou "Cavaleiro/Bastião".
func (p Path) Name() string {
	n := game.Classes[p.Class].Name
	if p.Spec != "" {
		n += "/" + game.SpecName(p.Spec)
	}
	return n
}

// Paths: as 8 combinações classe/caminho, na ordem da lista de caminhos.
func Paths() []Path {
	var out []Path
	for _, s := range game.SpecList {
		out = append(out, Path{s.Class, s.ID})
	}
	return out
}

// Classes: as 4 classes, na ordem da lista de caminhos.
func Classes() []string {
	var out []string
	for _, s := range game.SpecList {
		if !slices.Contains(out, s.Class) {
			out = append(out, s.Class)
		}
	}
	return out
}

// attrCycle: para onde vão os 3 pontos de atributo de cada nível (índices de game.Attrs:
// 0 Força, 1 Destreza, 2 Constituição, 3 Inteligência, 4 Sabedoria, 5 Espírito). Suposição:
// o jogador típico põe 2 no atributo de dano e 1 no de apoio.
var attrCycle = map[string][3]int{
	"knight":    {0, 2, 0}, // Força, Constituição, Força
	"barbarian": {0, 0, 2}, // Força, Força, Constituição
	"mage":      {3, 4, 3}, // Inteligência, Sabedoria, Inteligência
	"rogue":     {1, 0, 1}, // Destreza, Força, Destreza
}

// SpentAttrs: os pontos distribuídos até o nível.
func SpentAttrs(class string, level int) game.Attrs {
	var a game.Attrs
	cycle := attrCycle[class]
	for i := range game.AttrPointsTotal(level) {
		a = a.Add(game.UnitAttr(cycle[i%3]))
	}
	return a
}

// Ordem em que o jogador típico gasta os pontos de skill: cada ocorrência é +1 rank (as 2
// skills iniciais já começam no rank 1). Quem ainda não pode ser aprendida (nível do degrau,
// nó de cima) fica para depois e o ponto vai para a próxima da lista. Suposição: caçar
// sozinho, dano primeiro; defesa e utilidade com o que sobra.
var classOrder = map[string][]uint32{
	"knight": {
		game.SkillFencing, game.SkillFencing, game.SkillShieldBash, game.SkillWhirl, game.SkillFencing,
		game.SkillWhirl, game.SkillFencing, game.SkillShieldBash, game.SkillWhirl, game.SkillFencing,
		game.SkillCharge, game.SkillCharge, game.SkillCharge, game.SkillJudgment,
	},
	"barbarian": {
		game.SkillBrutalStrike, game.SkillFury, game.SkillHeavyAxe, game.SkillBrutalStrike, game.SkillHeavyAxe,
		game.SkillFury, game.SkillHeavyAxe, game.SkillBloodthirst, game.SkillQuake, game.SkillFrenzy,
		game.SkillQuake, game.SkillFrenzy, game.SkillQuake, game.SkillFrenzy, game.SkillExecute,
		game.SkillHeavyAxe, game.SkillHeavyAxe, game.SkillBloodthirst, game.SkillBloodthirst,
	},
	"mage": {
		game.SkillFireball, game.SkillEmber, game.SkillFireball, game.SkillEmber, game.SkillClearMind,
		game.SkillEmber, game.SkillIceLance, game.SkillEmberRain, game.SkillEmberRain, game.SkillEmberRain,
		game.SkillIceLance, game.SkillIceLance, game.SkillMeteor, game.SkillEmber, game.SkillEmber,
		game.SkillClearMind, game.SkillClearMind,
	},
	"rogue": {
		game.SkillBackstab, game.SkillWeakSpot, game.SkillBackstab, game.SkillWeakSpot, game.SkillWeakSpot,
		game.SkillPoisonedBlade, game.SkillWeakSpot, game.SkillTwinBlades, game.SkillTwinBlades, game.SkillTwinBlades,
		game.SkillWeakSpot, game.SkillDeathMark, game.SkillPoisonedBlade, game.SkillToxicFlask, game.SkillToxicFlask,
		game.SkillPoisonedBlade,
	},
}

// fifthTier: a skill do 5º degrau (nível 20) que o jogador típico pega primeiro.
var fifthTier = map[string]uint32{
	"knight":    game.SkillHorizon,
	"barbarian": game.SkillSeismicAxe,
	"mage":      game.SkillInferno,
	"rogue":     game.SkillFatalBlow,
}

// fillers: para onde vão os pontos que sobram (defesa e sustento).
var fillers = map[string][]uint32{
	"knight":    {game.SkillIronSkin, game.SkillIronSkin, game.SkillIronSkin, game.SkillIronSkin, game.SkillIronSkin, game.SkillFortress, game.SkillFortress, game.SkillWarCry},
	"barbarian": {game.SkillThickHide, game.SkillThickHide, game.SkillThickHide, game.SkillThickHide, game.SkillThickHide},
	"mage":      {game.SkillIceArmor, game.SkillIceArmor, game.SkillIceArmor, game.SkillClearMind, game.SkillClearMind},
	"rogue":     {game.SkillToxicFlask, game.SkillPoisonedBlade, game.SkillPoisonedBlade, game.SkillDodge, game.SkillDodge, game.SkillDodge},
}

// skillPlan: a ordem completa de uma classe com o caminho. Do 20 em diante: 5º degrau,
// as 3 do caminho no rank 1 (cada uma quando o degrau libera) e depois os ranks 2 e 3.
func skillPlan(class, spec string) []uint32 {
	plan := slices.Clone(classOrder[class])
	f := fifthTier[class]
	tree := game.SpecTree(spec)
	plan = append(plan, f)
	for _, s := range tree {
		plan = append(plan, s.ID)
	}
	plan = append(plan, f, f)
	for _, s := range tree {
		plan = append(plan, s.ID, s.ID)
	}
	return append(plan, fillers[class]...)
}

// Ranks: a árvore do jogador típico no nível (o caminho só vale do 20 em diante).
func Ranks(class, spec string, level int) map[uint32]int {
	if level < game.SpecLevel {
		spec = ""
	}
	ranks := game.StarterRanks(class)
	points := game.SkillPointsTotal(level) - game.RanksUsed(ranks)
	// Cada ocorrência no plano vira um rank alvo (a 2ª ocorrência de uma skill inicial é o
	// rank 3, por exemplo).
	type step struct {
		id     uint32
		target int
	}
	var steps []step
	count := map[uint32]int{}
	for _, id := range skillPlan(class, spec) {
		if count[id] == 0 {
			count[id] = ranks[id]
		}
		count[id]++
		if count[id] <= game.Skills[id].MaxRank {
			steps = append(steps, step{id, count[id]})
		}
	}
	for points > 0 {
		learned := false
		for _, st := range steps {
			s := game.Skills[st.id]
			if ranks[st.id] != st.target-1 || level < s.LevelReq() || (s.Spec != "" && s.Spec != spec) {
				continue
			}
			if p := s.Parent(); p != nil && ranks[p.ID] < 1 {
				continue
			}
			ranks[st.id] = st.target
			points--
			learned = true
			break
		}
		if !learned {
			break
		}
	}
	return ranks
}

// ---------------------------------------------------------------------------------------
// Equipamento típico
// ---------------------------------------------------------------------------------------

// Suposição do equipamento típico: em cada encaixe, a melhor peça comum ou boa (as que a
// loja vende) que a classe pode usar no nível, pelo poder do item. Raras, épicas e
// lendárias são drop ou criação e ficam fora da conta.
var typicalGrades = []game.Grade{game.GradeCommon, game.GradeGood}

// Piece é uma peça equipada.
type Piece struct {
	Item    *game.ItemDef
	Enchant int
}

// TypicalGear: o equipamento típico da classe no nível, todas as peças com +enchant.
func TypicalGear(class string, level, enchant int) []Piece {
	best := map[game.ItemKind]*game.ItemDef{}
	for _, d := range Gear() {
		if !slices.Contains(typicalGrades, d.Grade) || d.ReqLevel > level || !d.UsableBy(class) {
			continue
		}
		if b := best[d.Kind]; b == nil || better(d, b) {
			best[d.Kind] = d
		}
	}
	var out []Piece
	for _, k := range []game.ItemKind{game.KindWeapon, game.KindShield, game.KindArmor, game.KindHelmet, game.KindLegs,
		game.KindGloves, game.KindBoots, game.KindCape, game.KindNecklace, game.KindEarring, game.KindRing} {
		d := best[k]
		if d == nil {
			continue
		}
		// Brincos e anéis: dois encaixes (adagas são duas peças, mas um item só).
		for range len(d.Slots()) {
			out = append(out, Piece{d, enchant})
		}
	}
	return out
}

// GearBonus soma os bônus das peças (a mesma conta da zona).
func GearBonus(gear []Piece) game.Stats {
	var b game.Stats
	for _, p := range gear {
		b = game.AddItemBonus(b, p.Item, p.Enchant)
	}
	return b
}

func weaponType(gear []Piece) string {
	for _, p := range gear {
		if p.Item.Kind == game.KindWeapon {
			return p.Item.WeaponType
		}
	}
	return ""
}

// Build é o personagem típico montado: atributos finais, árvore e equipamento.
type Build struct {
	Path    Path
	Level   int
	Enchant int
	Attrs   game.Attrs
	Ranks   map[uint32]int
	Gear    []Piece
	Stats   game.Stats
	// Bestiary: bônus de dano do Bestiário contra os monstros da caça (0 = nenhum; o teto
	// game.BestiaryDamageCap no veterano, ver bestiary.go).
	Bestiary float64
}

// NewBuild monta o personagem típico (o caminho só vale do 20 em diante).
func NewBuild(p Path, level, enchant int) *Build {
	spec := p.Spec
	if level < game.SpecLevel {
		spec = ""
	}
	b := &Build{Path: Path{p.Class, spec}, Level: level, Enchant: enchant,
		Attrs: SpentAttrs(p.Class, level), Ranks: Ranks(p.Class, spec, level), Gear: TypicalGear(p.Class, level, enchant)}
	if err := game.ValidateRanks(p.Class, spec, level, b.Ranks); err != nil {
		panic(fmt.Sprintf("árvore típica inválida (%s nv %d): %v", p.Name(), level, err))
	}
	b.Stats = game.CharacterStats(p.Class, spec, level, b.Attrs, GearBonus(b.Gear), b.Ranks, weaponType(b.Gear))
	return b
}

func (b *Build) rank(id uint32) int { return b.Ranks[id] }

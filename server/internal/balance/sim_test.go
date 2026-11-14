package balance

import (
	"math"
	"reflect"
	"testing"

	"erevia/server/internal/game"
)

func TestTypicalBuildsFollowTheRules(t *testing.T) {
	for _, p := range Paths() {
		for l := 1; l <= game.MaxLevel; l++ {
			b := NewBuild(p, l, 0) // valida a árvore (ValidateRanks) e entra em pânico se não fecha
			if b.Attrs.Sum() != game.AttrPointsTotal(l) {
				t.Errorf("%s nv %d: %d pontos de atributo, want %d", p.Name(), l, b.Attrs.Sum(), game.AttrPointsTotal(l))
			}
			if used := game.RanksUsed(b.Ranks); used != game.SkillPointsTotal(l) {
				t.Errorf("%s nv %d: %d pontos de skill gastos de %d", p.Name(), l, used, game.SkillPointsTotal(l))
			}
			if (l < game.SpecLevel) != (b.Path.Spec == "") {
				t.Errorf("%s nv %d: caminho %q", p.Name(), l, b.Path.Spec)
			}
			if len(rotation(b)) == 0 {
				t.Errorf("%s nv %d: rotação vazia", p.Name(), l)
			}
		}
	}
	// No máximo todas as skills do caminho estão aprendidas.
	for _, p := range Paths() {
		b := NewBuild(p, game.MaxLevel, 0)
		for _, s := range game.SpecTree(p.Spec) {
			if b.rank(s.ID) < 1 {
				t.Errorf("%s: não aprendeu %s no %d", p.Name(), s.Name, game.MaxLevel)
			}
		}
	}
}

func TestTypicalGear(t *testing.T) {
	for _, c := range Classes() {
		for _, l := range []int{1, 5, 6, 29, 30, 34, 35, game.MaxLevel} {
			gear := TypicalGear(c, l, 0)
			want := game.GradeCommon
			if l >= 6 {
				want = game.GradeGood
			}
			// Do 30 em diante, a arma e a armadura são as ponte comuns do nível (30 ou 35).
			bridge := 0
			for _, lvl := range game.BridgeLevels {
				if l >= lvl {
					bridge = lvl
				}
			}
			rings, shield := 0, false
			for _, p := range gear {
				w := want
				isBridge := p.Item.Kind == game.KindWeapon || p.Item.Kind == game.KindArmor
				if bridge > 0 && isBridge {
					w = game.GradeCommon
					if p.Item.ID != game.BridgeWeaponID(c, bridge, w) && p.Item.ID != game.BridgeArmorID(c, bridge, w) {
						t.Errorf("%s nv %d: %s, want a peça ponte do %d", c, l, p.Item.ID, bridge)
					}
				} else if p.Item.Bridge {
					t.Errorf("%s nv %d: peça ponte %s fora da arma e da armadura", c, l, p.Item.ID)
				}
				if p.Item.Grade != w || p.Item.ReqLevel > l || !p.Item.UsableBy(c) {
					t.Errorf("%s nv %d: %s (raridade %d, nível %d)", c, l, p.Item.ID, p.Item.Grade, p.Item.ReqLevel)
				}
				if p.Item.Kind == game.KindRing {
					rings++
				}
				shield = shield || p.Item.Kind == game.KindShield
			}
			if rings != 2 || shield != (c == "knight" || c == "barbarian") {
				t.Errorf("%s nv %d: %d anéis, escudo %v", c, l, rings, shield)
			}
		}
	}
	// As peças que só mudam o visual não roubam o lugar da que a loja vende primeiro.
	if w := TypicalGear("knight", 1, 0)[0].Item.ID; w != "sword_common" {
		t.Errorf("arma comum do cavaleiro: %s", w)
	}
	// +3 soma o encantamento da zona.
	b0, b3 := GearBonus(TypicalGear("mage", 10, 0)), GearBonus(TypicalGear("mage", 10, 3))
	if b3.MAtk-b0.MAtk != game.EnchantBonus(game.Items["staff_good"], 3).MAtk {
		t.Errorf("cajado +3: %v → %v", b0.MAtk, b3.MAtk)
	}
}

func TestHuntRegions(t *testing.T) {
	cases := map[int][]string{
		1: {"skeleton_minion"}, 5: {"skeleton_minion"}, 6: {"skeleton_warrior", "skeleton_rogue"},
		12: {"skeleton_warrior", "skeleton_rogue"}, 13: {"skeleton_mage", "skeleton_warrior"},
		19: {"skeleton_mage", "skeleton_warrior"}, 20: {"stone_golem"}, 29: {"stone_golem"},
		// Do 30 ao 40 (tunnel#149): Fundo do Vale, Vigília Ossária e Confins Partidos.
		30: {"ancient_golem"}, 32: {"ancient_golem"}, 33: {"ossuary_knight", "ossuary_devotee"},
		34: {"ossuary_knight", "ossuary_devotee"}, 35: {"broken_reaper", "broken_arcanist"},
		game.MaxLevel - 1: {"broken_reaper", "broken_arcanist"}, game.MaxLevel: {"broken_reaper", "broken_arcanist"},
	}
	for l, want := range cases {
		h := HuntFor(l)
		var got []string
		for _, m := range h.Monsters {
			got = append(got, m.Def.ID)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("nível %d caça %v, want %v", l, got, want)
		}
		if l < h.LevelMin || l > h.LevelMax || h.TravelSeconds(5) <= 0 || h.MinCycle() <= 0 {
			t.Errorf("nível %d: região %+v", l, h)
		}
	}
}

// Sem skills, a luta é só o ataque básico: o tempo para matar sai da conta à mão.
func TestBasicAttackFightMatchesFormula(t *testing.T) {
	b := NewBuild(Path{Class: "knight"}, 10, 0)
	b.Ranks = map[uint32]int{}
	def := game.Monsters["skeleton_warrior"]
	m := game.MonsterStats(def, 10)
	hit := game.ExpectedHit(game.HitInput{Atk: b.Stats.PAtk, Def: m.PDef, Mult: 1, Crit: b.Stats.Crit,
		CritMult: b.Stats.CritDamage, Accuracy: b.Stats.Accuracy, Evasion: m.Evasion})
	hits := math.Ceil(m.MaxHP / hit)
	want := tickDT + (hits-1)*math.Ceil(b.Stats.AtkInterval/tickDT-timeEps)*tickDT
	if got := KillFight(b, def, 10, false).Time; math.Abs(got-want) > 1e-6 {
		t.Errorf("tempo para matar %.3f s, want %.3f (%v golpes de %.1f)", got, want, hits, hit)
	}
}

func TestSimulationIsDeterministicAndSane(t *testing.T) {
	for _, p := range []Path{{"knight", "bastion"}, {"mage", "occultist"}, {"rogue", "hunter"}, {"barbarian", "bloodrager"}} {
		for _, l := range []int{3, 15, 25} {
			a, b := Simulate(p, l, 0, false), Simulate(p, l, 0, false)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("%s nv %d: duas simulações diferentes", p.Name(), l)
			}
			if a.Died || a.TTK <= 0 || a.DPS <= 0 || a.XPH <= 0 || math.IsInf(a.LevelTime, 1) {
				t.Errorf("%s nv %d: %+v", p.Name(), l, a)
			}
			// +3 não mata mais devagar; de noite o monstro bate mais forte e vale mais XP.
			if e := Simulate(p, l, game.EnchantSafe, false); e.DPS < a.DPS || e.TTK > a.TTK*1.1 {
				t.Errorf("%s nv %d: +3 pior que +0 (%.1f/%.2f contra %.1f/%.2f)", p.Name(), l, e.DPS, e.TTK, a.DPS, a.TTK)
			}
			if n := Simulate(p, l, 0, true); n.XPKill <= a.XPKill || n.TTDNoPot > a.TTDNoPot {
				t.Errorf("%s nv %d: noite sem efeito (%v/%v)", p.Name(), l, n.XPKill, n.TTDNoPot)
			}
		}
		if tmax := TimeToMax(Progression(p, 0, false)); tmax <= 0 || math.IsInf(tmax, 1) {
			t.Errorf("%s: tempo até o %d = %v", p.Name(), game.MaxLevel, tmax)
		}
	}
}

func TestPowerBasics(t *testing.T) {
	if ItemPower(game.Items["sword_good"], 3) <= ItemPower(game.Items["sword_good"], 0) {
		t.Error("encantar deveria somar poder")
	}
	for _, fam := range Families() {
		ladder := Ladder(fam)
		if len(ladder) < 2 {
			t.Errorf("%s: escada com %d peças", fam, len(ladder))
		}
		for i := 1; i < len(ladder); i++ {
			if ladder[i].Grade <= ladder[i-1].Grade || ItemPower(ladder[i], 0) <= ItemPower(ladder[i-1], 0) {
				t.Errorf("%s: %s não vale mais que %s", fam, ladder[i].ID, ladder[i-1].ID)
			}
		}
	}
	for _, d := range Gear() {
		if ItemBudget(d) <= 0 {
			t.Errorf("%s sem orçamento", d.ID)
		}
	}
}

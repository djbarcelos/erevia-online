package game

import (
	"math"
	"slices"
	"testing"
)

func TestSpecDataIsConsistent(t *testing.T) {
	if len(SpecList) != 8 {
		t.Fatalf("%d evoluções, want 8", len(SpecList))
	}
	// Cada árvore em uso (a base de cada classe e a de cada evolução) tem ícones distintos.
	trees := map[string][]*SkillDef{}
	for class := range Classes {
		trees[class] = ClassTree(class)
	}
	for _, sp := range SpecList {
		trees[sp.ID] = EvolutionTree(sp.ID)
	}
	for name, tree := range trees {
		fx := map[string]string{}
		for _, s := range tree {
			if o := fx[s.FX]; o != "" {
				t.Errorf("%s: %s e %s com o mesmo FX %q", name, o, s.Name, s.FX)
			}
			fx[s.FX] = s.Name
		}
	}
	for class := range Classes {
		specs := ClassSpecs(class)
		if len(specs) != 2 {
			t.Fatalf("%s: %d evoluções, want 2", class, len(specs))
		}
		for _, sp := range specs {
			if sp.Name == "" || sp.Description == "" || len(sp.Color) != 7 || sp.Bonus.Sum() != 12 || !sp.Bonus.Valid() || sp.BonusText() == "" {
				t.Errorf("%s: dados incompletos: %+v", sp.ID, sp)
			}
			if len([]rune(sp.Name)) > 16 {
				t.Errorf("%s: nome longo demais para o título sob o nome", sp.Name)
			}
			for _, b := range sp.Branches {
				if b.Name == "" || b.Description == "" || len(b.Color) != 7 {
					t.Errorf("%s: ramo incompleto %+v", sp.ID, b)
				}
			}
			// As 2 iniciais: ativas da evolução, no marco 20, sem requisito.
			for _, id := range sp.StarterSkills {
				s := Skills[id]
				if s == nil || s.Spec != sp.ID || s.IsPassive() || s.LevelReq() != SpecLevel || s.BranchPoints != 0 || len(s.Requires) != 0 {
					t.Errorf("%s: inicial %d inválida", sp.ID, id)
				}
			}
			if err := ValidateRanks(class, sp.ID, SpecLevel, StarterRanks(class, sp.ID)); err != nil {
				t.Errorf("%s: iniciais inválidas no 20: %v", sp.ID, err)
			}
			tree := EvolutionTree(sp.ID)
			branches := map[int]bool{}
			for _, s := range tree {
				branches[s.Branch] = true
				if s.Class != class || s.Branch < 0 || s.Branch > 2 || !slices.Contains(EvolutionMilestones, s.Level) || s.Tier != (s.Level-20)/5 {
					t.Errorf("%s: lugar errado (classe %s, ramo %d, marco %d)", s.Name, s.Class, s.Branch, s.Level)
				}
				if s.BranchPoints != MilestoneBranchPoints(s.Level) {
					t.Errorf("%s: %d pontos no ramo no marco %d", s.Name, s.BranchPoints, s.Level)
				}
				if s.Parent() != nil {
					t.Errorf("%s: a árvore da evolução não usa nó de cima", s.Name)
				}
				if s.ID >= 1000 || s.MaxRank < 1 || s.Text == nil || s.Text(1) == "" || s.FX == "" || s.Description == "" {
					t.Errorf("%s: incompleta", s.Name)
				}
				for _, r := range s.Requires {
					if o := Skills[r.ID]; o == nil || o.Spec != sp.ID || r.Rank < 1 || r.Rank > o.MaxRank {
						t.Errorf("%s: requisito %d fora da árvore", s.Name, r.ID)
					}
				}
				if s.Modifies != 0 {
					if o := Skills[s.Modifies]; o == nil || o.Spec != sp.ID || o.IsPassive() || s.Mod == nil || s.Node != NodeModifier {
						t.Errorf("%s: modificador inválido", s.Name)
					}
				}
				if s.IsPassive() && s.Node != NodePassive && s.Node != NodeKey || !s.IsPassive() && s.Node == NodePassive {
					t.Errorf("%s: tipo de nó %d", s.Name, s.Node)
				}
				if (s.Node == NodeKey || s.Node == NodeSupreme) != (s.MaxRank == 1 && s.Level >= 40) {
					t.Errorf("%s: chave com %d ranks no marco %d", s.Name, s.MaxRank, s.Level)
				}
				if s.IsPassive() || s.Node == NodeModifier {
					continue
				}
				if s.Target == 0 || s.MP.At(1)+SkillResCost(s) <= 0 || s.CD.At(1) <= 0 {
					t.Errorf("%s: ativa sem alvo, custo ou recarga", s.Name)
				}
				if s.Mult.At(1) <= 0 && s.OnTarget == nil && s.OnSelf == nil && s.Banner == nil && s.Zone == nil &&
					s.CleanseAllies == (Scale{}) && !s.RenewBlessings {
					t.Errorf("%s: não faz nada", s.Name)
				}
			}
			if len(branches) != 3 {
				t.Errorf("%s: skills em %d ramos, want 3", sp.ID, len(branches))
			}
		}
	}
	// As skills do degrau 20 de antes e das especializações (49–84) mudaram de árvore, com o
	// mesmo id.
	for id := SkillBulwark; id <= SkillHuntRain; id++ {
		if s := Skills[id]; s == nil || s.Spec == "" || SpecFor(s.Class, s.Spec) == nil {
			t.Errorf("skill %d fora das evoluções", id)
		}
	}
	if (&SpecDef{Bonus: Attrs{Str: 6, Con: 6}}).BonusText() != "+6 Força e +6 Constituição" {
		t.Error("texto do bônus")
	}
}

func TestValidateRanksWithSpec(t *testing.T) {
	bastion := func(extra map[uint32]int) map[uint32]int {
		r := StarterRanks("knight", "bastion")
		for id, n := range extra {
			r[id] = n
		}
		return r
	}
	if ValidateRanks("knight", "", 30, map[uint32]int{SkillShieldBash: 1, SkillFortress: 1, SkillBastionDefy: 1}) == nil {
		t.Error("skill da evolução sem evoluir")
	}
	if ValidateRanks("knight", "bastion", 30, bastion(map[uint32]int{SkillShieldBash: 1})) == nil {
		t.Error("skill da base depois de evoluir")
	}
	if ValidateRanks("knight", "herald", 30, bastion(nil)) == nil {
		t.Error("skill da outra evolução")
	}
	if err := ValidateRanks("knight", "bastion", 20, bastion(map[uint32]int{SkillBastionDefy: 3, SkillBulwark: 3})); err != nil {
		t.Errorf("Desafio e Baluarte no 20: %v", err)
	}
	// Último Bastião: marco 35 e 10 pontos no ramo Muralha (Guarda 3 + Vigor 3 = 6).
	wall := bastion(map[uint32]int{SkillBastionGuard: 3, SkillBastionVigor: 3, SkillBastionLast: 1})
	if ValidateRanks("knight", "bastion", 34, wall) == nil {
		t.Error("marco 35 antes do 35")
	}
	if err := ValidateRanks("knight", "bastion", 35, wall); err == nil {
		t.Error("marco 35 com 6 pontos no ramo")
	}
	// Pontos: 21 no 20 (as 2 iniciais contam).
	full := bastion(map[uint32]int{SkillBastionGuard: 3, SkillBastionBash: 3, SkillBastionVigor: 3, SkillAegis: 3, SkillBastionDefy: 3, SkillBulwark: 3})
	if RanksUsed(full) != 18 || ValidateRanks("knight", "bastion", 20, full) != nil {
		t.Errorf("18 ranks no 20: %v", ValidateRanks("knight", "bastion", 20, full))
	}
	if ValidateRanks("knight", "bastion", 30, bastion(map[uint32]int{SkillElemLance: 1})) == nil {
		t.Error("skill de evolução de outra classe")
	}
	if SpecFor("knight", "elementalist") != nil || SpecFor("mage", "elementalist") == nil {
		t.Error("SpecFor deveria conferir a classe")
	}
	if SpecBonus("knight", "bastion") != (Attrs{Con: 12}) || SpecBonus("knight", "") != (Attrs{}) {
		t.Error("SpecBonus")
	}
	if EvolutionChangeCost(20) != 50_000 || RespecCost(40) != 20_000 {
		t.Error("custos de troca e de redistribuição")
	}
}

func TestEvolvedHotbar(t *testing.T) {
	cases := []struct {
		class, spec string
		from, to    uint32
	}{
		{"knight", "bastion", SkillShieldBash, SkillBastionBash},
		{"knight", "bastion", SkillFortress, SkillBastionGuard},
		{"knight", "herald", SkillShieldBash, SkillHeraldStrike},
		{"barbarian", "warlord", SkillFury, SkillWarlordCry},
		{"mage", "occultist", SkillFireball, SkillOccDrain},
		{"rogue", "hunter", SkillShadowStep, SkillHuntKnife},
		{"rogue", "cutthroat", SkillBackstab, SkillCutBackstab},
		{"rogue", "cutthroat", SkillTwinBlades, 0},
	}
	for _, c := range cases {
		if got := EvolvedHotbarID(c.class, c.spec, c.from); got != c.to {
			t.Errorf("%s: %d → %d, want %d", c.spec, c.from, got, c.to)
		}
	}
}

func TestEffectiveSkillModifiers(t *testing.T) {
	base := &SkillDef{ID: 9001, Mult: Scale{2, 0.5}, CD: Scale{10, 0}, MP: Scale{20, 0}, Hits: 1,
		OnTarget: &EffectSpec{Kind: EffectBurn, Duration: Scale{3, 0}}}
	mod := &SkillDef{ID: 9002, Modifies: 9001, Node: NodeModifier, MaxRank: 3,
		Mod: &SkillMod{MultPct: Scale{0.1, 0.1}, CD: Scale{-1, -1}, EffectDur: Scale{1, 1}, Hits: [4]int{0, 0, 0, 1}}}
	Skills[base.ID], Skills[mod.ID] = base, mod
	modifiersOf[base.ID] = []*SkillDef{mod}
	defer func() {
		delete(Skills, base.ID)
		delete(Skills, mod.ID)
		delete(modifiersOf, base.ID)
	}()
	if EffectiveSkill(base, map[uint32]int{9001: 1}) != base {
		t.Fatal("sem modificador deveria devolver a própria skill")
	}
	got := EffectiveSkill(base, map[uint32]int{9001: 1, 9002: 3})
	if got == base || math.Abs(got.Mult.At(1)-2*1.3) > 1e-9 || got.CD.At(1) != 7 || got.OnTarget.Duration.At(1) != 6 || got.Hits != 2 {
		t.Errorf("rank 3: mult %v cd %v dur %v hits %d", got.Mult.At(1), got.CD.At(1), got.OnTarget.Duration.At(1), got.Hits)
	}
	if base.OnTarget.Duration.At(1) != 3 || base.Mult.At(1) != 2 {
		t.Error("o modificador alterou a skill original")
	}
}

// completeEvolutions: as evoluções com a árvore inteira (fases de conteúdo já feitas).
var completeEvolutions = []string{"bastion", "herald"}

// A árvore completa tem folga: pelo menos 1,4× os pontos do 40 e 26 ranks no marco 20;
// uma chave por ramo no 40, que exige 15 pontos no ramo.
func TestCompleteTreesHaveRoom(t *testing.T) {
	for _, id := range completeEvolutions {
		total, at20 := 0, 0
		keys := map[int]int{}
		for _, s := range EvolutionTree(id) {
			total += s.MaxRank
			if s.Level == 20 {
				at20 += s.MaxRank
			}
			if s.Node == NodeKey {
				keys[s.Branch]++
			}
		}
		if pts := SkillPointsTotal(40, true); float64(total) < 1.4*float64(pts) {
			t.Errorf("%s: %d ranks para %d pontos no 40", id, total, pts)
		}
		if at20 < 26 {
			t.Errorf("%s: %d ranks no marco 20, want ≥ 26", id, at20)
		}
		if len(keys) != 3 || keys[0] != 1 || keys[1] != 1 || keys[2] != 1 {
			t.Errorf("%s: chaves por ramo %v", id, keys)
		}
	}
}

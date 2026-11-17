package game

import "testing"

func TestSpecDataIsConsistent(t *testing.T) {
	if len(SpecList) != 8 {
		t.Fatalf("%d especializações, want 8", len(SpecList))
	}
	fx := map[string]string{}
	for _, s := range Skills {
		fx[s.FX] = s.Name
	}
	if len(fx) != len(Skills) {
		t.Errorf("FX repetido: %d ids para %d skills", len(fx), len(Skills))
	}
	for class := range Classes {
		specs := ClassSpecs(class)
		if len(specs) != 2 {
			t.Fatalf("%s: %d caminhos, want 2", class, len(specs))
		}
		for _, sp := range specs {
			if sp.Name == "" || sp.Description == "" || len(sp.Color) != 7 || sp.Bonus.Sum() != 12 || !sp.Bonus.Valid() || sp.BonusText() == "" {
				t.Errorf("%s: dados incompletos: %+v", sp.ID, sp)
			}
			if len([]rune(sp.Name)) > 14 {
				t.Errorf("%s: nome longo demais para o título sob o nome", sp.Name)
			}
			tree := SpecTree(sp.ID)
			if len(tree) != 3 {
				t.Fatalf("%s: %d skills, want 3", sp.ID, len(tree))
			}
			for i, s := range tree {
				if s.Class != class || s.Spec != sp.ID || s.Branch != SpecBranch || s.Tier != i || s.LevelReq() != SpecTierLevels[i] {
					t.Errorf("%s: lugar errado (classe %s, ramo %d, degrau %d, nível %d)", s.Name, s.Class, s.Branch, s.Tier, s.LevelReq())
				}
				if s.ID >= 1000 || s.MaxRank < 1 || s.Text == nil || s.Text(1) == "" || s.FX == "" || s.Description == "" {
					t.Errorf("%s: incompleta", s.Name)
				}
				if p := s.Parent(); i > 0 && (p == nil || p.ID != tree[i-1].ID) {
					t.Errorf("%s: o pai deveria ser %s", s.Name, tree[i-1].Name)
				} else if i == 0 && p != nil {
					t.Errorf("%s: a raiz do caminho não tem pai", s.Name)
				}
				if s.IsPassive() {
					continue
				}
				if s.Target == 0 || s.MP.At(1) <= 0 || s.CD.At(1) <= 0 {
					t.Errorf("%s: ativa sem alvo, custo ou recarga", s.Name)
				}
				if s.Mult.At(1) <= 0 && s.OnTarget == nil && s.OnSelf == nil && s.Banner == nil {
					t.Errorf("%s: não faz nada", s.Name)
				}
				if s.MaxRank > 1 && s.Text(1) == s.Text(s.MaxRank) {
					t.Errorf("%s: o texto deveria mudar com o rank", s.Name)
				}
			}
		}
	}
	if (&SpecDef{Bonus: Attrs{Str: 6, Con: 6}}).BonusText() != "+6 Força e +6 Constituição" {
		t.Error("texto do bônus")
	}
}

func TestValidateRanksWithSpec(t *testing.T) {
	base := func(extra map[uint32]int) map[uint32]int {
		r := map[uint32]int{SkillShieldBash: 1, SkillFortress: 1}
		for id, n := range extra {
			r[id] = n
		}
		return r
	}
	if ValidateRanks("knight", "", 30, base(map[uint32]int{SkillBastionDefy: 1})) == nil {
		t.Error("skill de caminho sem especialização")
	}
	if ValidateRanks("knight", "herald", 30, base(map[uint32]int{SkillBastionDefy: 1})) == nil {
		t.Error("skill do outro caminho")
	}
	if err := ValidateRanks("knight", "bastion", 20, base(map[uint32]int{SkillBastionDefy: 3})); err != nil {
		t.Errorf("Desafio do Bastião no 20: %v", err)
	}
	if ValidateRanks("knight", "bastion", 23, base(map[uint32]int{SkillBastionDefy: 1, SkillBastionVigor: 1})) == nil {
		t.Error("2º degrau do caminho antes do nível 24")
	}
	if ValidateRanks("knight", "bastion", 30, base(map[uint32]int{SkillBastionVigor: 1})) == nil {
		t.Error("2º degrau sem o 1º")
	}
	if err := ValidateRanks("knight", "bastion", 28, base(map[uint32]int{SkillBastionDefy: 1, SkillBastionVigor: 1, SkillBastionLast: 1})); err != nil {
		t.Errorf("caminho inteiro no 28: %v", err)
	}
	if ValidateRanks("knight", "elementalist", 30, base(map[uint32]int{SkillElemLance: 1})) == nil {
		t.Error("skill de caminho de outra classe")
	}
	if SpecFor("knight", "elementalist") != nil || SpecFor("mage", "elementalist") == nil {
		t.Error("SpecFor deveria conferir a classe")
	}
	if SpecBonus("knight", "bastion") != (Attrs{Con: 12}) || SpecBonus("knight", "") != (Attrs{}) {
		t.Error("SpecBonus")
	}
}

package game

import "testing"

// Uma skill passiva sintética de qualquer classe, com um tipo de mecânica, entra no total.
func TestPassiveKindsFromAnySkill(t *testing.T) {
	const id = 9_000_001
	Skills[id] = &SkillDef{ID: id, Class: "rogue", Name: "Teste", MaxRank: 3, Passive: PassiveLifesteal, PValue: Scale{0.02, 0.01}}
	defer delete(Skills, id)

	got := PassiveTotals(map[uint32]int{id: 3})
	if want := 0.04; got.Of(PassiveLifesteal) < want-1e-9 || got.Of(PassiveLifesteal) > want+1e-9 {
		t.Fatalf("lifesteal total = %v, want %v", got.Of(PassiveLifesteal), want)
	}
	if PassiveTotals(map[uint32]int{id: 0}).Has(PassiveLifesteal) {
		t.Error("rank 0 não conta")
	}
	// Duas skills do mesmo tipo somam.
	two := PassiveTotals(map[uint32]int{id: 1, SkillBloodthirst: 1})
	if two.Of(PassiveLifesteal) <= PassiveTotals(map[uint32]int{SkillBloodthirst: 1}).Of(PassiveLifesteal) {
		t.Error("skills do mesmo tipo devem somar")
	}
}

func TestSkillAvailable(t *testing.T) {
	if SkillAvailable(nil, "knight", "") {
		t.Error("nil não está disponível")
	}
	s := &SkillDef{Class: "knight", Spec: "x"}
	if !SkillAvailable(s, "knight", "x") || SkillAvailable(s, "knight", "y") || SkillAvailable(s, "mage", "x") {
		t.Error("classe e spec")
	}
	if !SkillAvailable(&SkillDef{Class: "knight"}, "knight", "qualquer") {
		t.Error("skill sem spec vale para toda spec")
	}
}

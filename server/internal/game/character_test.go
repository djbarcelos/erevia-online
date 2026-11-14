package game

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestExpectedHitMatchesRollHit(t *testing.T) {
	cases := []HitInput{
		{Atk: 100, Def: 100, Mult: 1, Crit: 0.2, Accuracy: 100, Evasion: 50},
		{Atk: 80, Def: 30, Mult: 2.5, Crit: 0.05, CritMult: 2.4, Accuracy: 90, Evasion: 110},
		{Atk: 60, Def: 200, Mult: 1, Crit: 0.75, CritMult: 2.3, Accuracy: 120, Evasion: 40}, // crítico acima de 1 vira 1
		{Atk: 45, Def: 10, Mult: 4, NoMiss: true, Crit: 0.1, Accuracy: 0, Evasion: 500},
		{Atk: 50, Def: 0, Mult: 1, ForceCrit: true, CritMult: 2.5, Evasion: 1000},
		{Atk: 120, Def: 60, Mult: 1, Crit: 1.4, Accuracy: 95, Evasion: 95},
	}
	rng := rand.New(rand.NewPCG(11, 13))
	for i, in := range cases {
		const n = 200000
		var sum float64
		for range n {
			sum += RollHit(rng, in).Damage
		}
		mc, want := sum/n, ExpectedHit(in)
		if math.Abs(mc-want)/want > 0.01 {
			t.Errorf("caso %d: média sorteada %.3f, fórmula %.3f", i, mc, want)
		}
	}
	if ExpectedHit(HitInput{Atk: 1, Def: 10000, Mult: 1, NoMiss: true}) != 1 {
		t.Error("piso de 1 de dano")
	}
}

func TestCharacterStatsPipeline(t *testing.T) {
	// Sem pontos, sem equipamento e sem passivas: a classe no nível com a regeneração.
	for class := range Classes {
		got := CharacterStats(class, "", 9, Attrs{}, Stats{}, nil, "")
		want := FinishRegen(ApplyAttrs(ClassStats(class, 9), class, 9, ClassBaseAttrs[class]), class, ClassBaseAttrs[class])
		if got != want {
			t.Errorf("%s: %+v, want %+v", class, got, want)
		}
	}
	// Esgrima só com espada; Mente Clara multiplica a mana regenerada.
	ranks := map[uint32]int{SkillFencing: 5, SkillClearMind: 2}
	sword := CharacterStats("knight", "", 10, Attrs{}, Stats{}, ranks, WeaponSword)
	none := CharacterStats("knight", "", 10, Attrs{}, Stats{}, ranks, "")
	if math.Abs(sword.PAtk/none.PAtk-1.15) > 1e-9 {
		t.Errorf("Esgrima 5: %.2f / %.2f", sword.PAtk, none.PAtk)
	}
	plain := CharacterStats("knight", "", 10, Attrs{}, Stats{}, nil, "")
	if math.Abs(none.MPRegen/plain.MPRegen-1.2) > 1e-9 {
		t.Errorf("Mente Clara 2: %.3f / %.3f", none.MPRegen, plain.MPRegen)
	}
	// Bônus da especialização entra como atributo.
	if CharacterStats("knight", "bastion", 20, Attrs{}, Stats{}, nil, "").MaxHP <= CharacterStats("knight", "", 20, Attrs{}, Stats{}, nil, "").MaxHP {
		t.Error("Bastião (+12 Constituição) deveria ter mais vida")
	}
	// Item +3: bônus do item e do encantamento.
	b := AddItemBonus(Stats{}, Items["sword_good"], 3)
	if b.PAtk != 14+EnchantBonus(Items["sword_good"], 3).PAtk {
		t.Errorf("Espada de Aço +3: %+v", b)
	}
	// Custo e recarga.
	fb := Skills[SkillFireball]
	if SkillMPCost(fb, 1, false) != 15 || SkillMPCost(fb, 1, true) != math.Round(15*OverloadMPCost) {
		t.Error("custo da Bola de Fogo")
	}
	if SkillCooldownFor(fb, 1, 1.5) != 2 || SkillCooldownFor(fb, 1, 0) != 3 {
		t.Error("recarga da Bola de Fogo")
	}
}

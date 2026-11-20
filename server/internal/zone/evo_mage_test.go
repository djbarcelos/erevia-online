package zone

import (
	"math"
	"testing"

	"erevia/server/internal/game"
)

// Testes das mecânicas das evoluções do Mago (docs/evolucoes/elementalist.md e occultist.md).

// Reação Elemental: o dobro em alvo queimado e lento, consumindo os dois, e +10 de
// Concentração.
func TestElementalReaction(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	p := evolved(z, 1, "mage", "elementalist", 30)
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectBurn, Duration: 5, Value: 1})
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectSlow, Duration: 5, Value: 0.3})
	p.pl.res.Value = 0
	z.skillHit(p, m, game.Skills[game.SkillElemReaction], 1, false)
	if m.has(game.EffectBurn, z.now) || m.has(game.EffectSlow, z.now) {
		t.Error("a Reação consome a queimadura e a lentidão")
	}
	if p.pl.res.Value < 10 {
		t.Errorf("Concentração da Reação: %v", p.pl.res.Value)
	}
}

// Coração do Vulcão: fogo em alvo queimado espalha a queimadura a até 3 m.
func TestVolcanoHeartSpreadsBurn(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0), fixed("skeleton_warrior", 20, 42, 0))
	ms := dummies(z, 40, 42)
	p := evolved(z, 1, "mage", "elementalist", 30)
	learn(z, p, map[uint32]int{game.SkillElemVolcano: 1})
	z.applyEffect(p, ms[0], &game.EffectDef{Kind: game.EffectBurn, Duration: 5, Value: 3})
	z.skillHit(p, ms[0], game.Skills[game.SkillElemFireball], 1, false)
	if !ms[1].has(game.EffectBurn, z.now) {
		t.Error("a queimadura deveria passar para o vizinho")
	}
}

// Coração do Inverno: golpe em alvo preso é crítico garantido; Frio Penetrante soma dano.
func TestWinterHeartAndBitingCold(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	p := evolved(z, 1, "mage", "elementalist", 30)
	learn(z, p, map[uint32]int{game.SkillElemFrostCore: 1, game.SkillElemBiting: 5})
	if in := z.hitInput(p, m, true, 1, 0); in.ForceCrit {
		t.Fatal("sem prender, sem crítico garantido")
	}
	base := z.hitInput(p, m, true, 1, 0).Mult
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectRoot, Duration: 3})
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectSlow, Duration: 3, Value: 0.3})
	in := z.hitInput(p, m, true, 1, 0)
	if !in.ForceCrit || math.Abs(in.Mult-base*1.1) > 1e-9 {
		t.Errorf("preso e lento: crítico %v, mult %v (want %v)", in.ForceCrit, in.Mult, base*1.1)
	}
}

// Prisão Etérea: o alvo não apanha nem age; não pega em chefe.
func TestEtherealPrison(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	p := evolved(z, 1, "mage", "occultist", 30)
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectBanish, Duration: 2})
	hp := m.hp
	z.damage(p, m, 500, nil)
	if m.hp != hp || !m.stunned(z.now) {
		t.Error("banido: sem dano e sem agir")
	}
	boss := *m.mon.def
	boss.Boss = true
	m.mon.def, m.effects = &boss, nil
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectBanish, Duration: 2})
	if m.has(game.EffectBanish, z.now) {
		t.Error("não funciona em chefe")
	}
}

// Fraqueza tira defesa; Agonia soma dano em amaldiçoado; Abismo Interior alonga.
func TestCursesWeaknessAgonyAbyss(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	p := evolved(z, 1, "mage", "occultist", 30)
	learn(z, p, map[uint32]int{game.SkillOccAgony: 5, game.SkillOccAbyssCore: 1})
	def := m.def(false, z.now)
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectArmorBreak, Duration: 8, Value: 0.2})
	if got := m.def(false, z.now); math.Abs(got-def*0.8) > 1e-9 {
		t.Errorf("Fraqueza: defesa %v, want %v", got, def*0.8)
	}
	if ef := m.effectRef(game.EffectArmorBreak, z.now); ef == nil || math.Abs(ef.until-z.now-12) > 1e-9 {
		t.Errorf("Abismo Interior: dura 12 s, %+v", ef)
	}
	in := z.hitInput(p, m, true, 1, 0)
	m.effects = nil
	if base := z.hitInput(p, m, true, 1, 0).Mult; math.Abs(in.Mult-base*1.1) > 1e-9 {
		t.Errorf("Agonia 5: %v, want %v", in.Mult, base*1.1)
	}
}

// Pandemia: as pragas e maldições do alvo passam para os vizinhos.
func TestPandemicSpreads(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0), fixed("skeleton_warrior", 20, 45, 0), fixed("skeleton_warrior", 20, 55, 0))
	ms := dummies(z, 40, 45, 55)
	p := evolved(z, 1, "mage", "occultist", 30)
	z.applyEffect(p, ms[0], &game.EffectDef{Kind: game.EffectPlague, Duration: 8, Value: 10})
	z.applyEffect(p, ms[0], &game.EffectDef{Kind: game.EffectWeaken, Duration: 8, Value: 0.2})
	p.pl.res.Value = 60
	z.castSkill(p, game.Skills[game.SkillOccPandemic], ms[0])
	if !ms[1].has(game.EffectPlague, z.now) || !ms[1].has(game.EffectWeaken, z.now) {
		t.Error("o vizinho deveria pegar a praga e a maldição")
	}
	if ms[2].has(game.EffectPlague, z.now) {
		t.Error("a 15 m, fora do alcance")
	}
	// Gasta 60; os efeitos espalhados contam como efeito novo e geram Carga de novo (teto de 25/s).
	if p.pl.res.Value > 25 {
		t.Errorf("Pandemia gasta 60 de Carga: %v", p.pl.res.Value)
	}
}

// Contágio e Carga Arcana: a praga que pula fica mais forte e gera Carga (até 3).
func TestContagionAndPlagueCharge(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0), fixed("skeleton_warrior", 20, 42, 0), fixed("skeleton_warrior", 20, 43, 0))
	ms := dummies(z, 40, 42, 43)
	p := evolved(z, 1, "mage", "occultist", 30)
	learn(z, p, map[uint32]int{game.SkillOccContagion: 5})
	z.applyEffect(p, ms[0], &game.EffectDef{Kind: game.EffectPlague, Duration: 8, Value: 10})
	p.pl.res.Value = 0
	z.kill(ms[0], p)
	if v, _ := ms[1].effect(game.EffectPlague, z.now); v != 12 {
		t.Errorf("praga com Contágio 5: %v, want 12", v)
	}
	if p.pl.res.Value < 6 {
		t.Errorf("Carga dos saltos: %v", p.pl.res.Value)
	}
}

// Nuvem Pestilenta: dano por pulso nos inimigos dentro.
func TestPestilentCloud(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "mage", "occultist", 30)
	learn(z, p, map[uint32]int{game.SkillOccCloud: 1})
	hp := m.hp
	z.castSkill(p, game.Skills[game.SkillOccCloud], nil)
	z.tickFor(2.1)
	if m.hp >= hp {
		t.Error("a nuvem deveria causar dano")
	}
}

// Rajada Concentrada e Fluxo Arcano: com 20 de Carga, a Rajada gasta e devolve mana.
func TestFocusedBlastAndArcaneFlow(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	p := evolved(z, 1, "mage", "occultist", 30)
	learn(z, p, map[uint32]int{game.SkillOccFocused: 1, game.SkillOccFlow: 3})
	blast := game.EffectiveSkill(game.Skills[game.SkillArcaneBlast], p.pl.ranks)
	if blast.ResBonusCost != 20 {
		t.Fatalf("modificador: %+v", blast.ResBonusCost)
	}
	p.pl.res.Value = 25
	p.mp = 100
	z.castSkill(p, blast, m)
	if p.pl.res.Value >= 25-20+game.ResourceOf("mage", "occultist").PerSecondCap {
		t.Errorf("Carga depois da Rajada Concentrada: %v", p.pl.res.Value)
	}
	if want := 100 - skillMP(p, blast) + p.stats.MaxMP*0.04*2; math.Abs(p.mp-want) > 1 {
		t.Errorf("mana com Fluxo Arcano: %v, want ~%v", p.mp, want)
	}
}

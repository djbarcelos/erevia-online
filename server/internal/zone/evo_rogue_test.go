package zone

import (
	"math"
	"testing"

	"erevia/server/internal/game"
)

// Testes das mecânicas das evoluções do Ladino (docs/evolucoes/cutthroat.md e hunter.md).

// Dança das Lâminas: a finalização devolve 30% do Combo gasto.
func TestBladeDanceRefund(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "rogue", "cutthroat", 30)
	learn(z, p, map[uint32]int{game.SkillCutMercy: 1, game.SkillCutDance: 1})
	p.pl.res.Value = 40
	z.castSkill(p, game.Skills[game.SkillCutMercy], m)
	if p.pl.res.Value < 12 { // 40 gasto, 12 de volta (mais o que o golpe gerou)
		t.Errorf("Combo depois da finalização: %v, want ≥ 12", p.pl.res.Value)
	}
}

// Véu Mortal: com o véu, matar deixa invisível.
func TestDeathVeil(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "rogue", "cutthroat", 30)
	p.pl.res.Value = 30
	z.castSkill(p, game.Skills[game.SkillCutVeil], nil)
	m.hp = 1
	z.damage(p, m, 10, nil)
	if !m.dead || !p.has(game.EffectInvisible, z.now) {
		t.Errorf("Véu Mortal: morto %v, invisível %v", m.dead, p.has(game.EffectInvisible, z.now))
	}
}

// Assassino das Sombras e Sombra Persistente: sair do Sumir dá Combo, velocidade e crítico
// mais forte.
func TestShadowAssassinReveal(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "rogue", "cutthroat", 30)
	learn(z, p, map[uint32]int{game.SkillCutVanish: 1, game.SkillCutLinger: 1, game.SkillCutShadowKill: 1})
	z.castSkill(p, game.EffectiveSkill(game.Skills[game.SkillCutVanish], p.pl.ranks), nil)
	if !p.has(game.EffectInvisible, z.now) || p.pl.revealSpeed != 0.2 {
		t.Fatalf("Sumir: invisível %v, velocidade ao sair %v", p.has(game.EffectInvisible, z.now), p.pl.revealSpeed)
	}
	in := z.hitInput(p, m, false, 1, 0)
	if !in.ForceCrit || in.CritMult != p.stats.CritDamage+game.ShadowAssassinCrit {
		t.Errorf("primeiro golpe: crítico %v, multiplicador %v", in.ForceCrit, in.CritMult)
	}
	z.revealIfHidden(p)
	if p.pl.res.Value < 25 || !p.has(game.EffectBuffSpeed, z.now) {
		t.Errorf("ao sair: Combo %v, velocidade %v", p.pl.res.Value, p.has(game.EffectBuffSpeed, z.now))
	}
}

// Sangrar: o crítico abre uma ferida; Execução Perfeita: a finalização que mata zera as
// recargas do Passo Sombrio e do Golpe das Sombras.
func TestCritBleedAndPerfectExecution(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "rogue", "cutthroat", 30)
	learn(z, p, map[uint32]int{game.SkillCutBleed: 1, game.SkillCutPerfect: 1, game.SkillCutMercy: 1})
	z.applyHit(p, m, 0, game.Hit{Damage: 10, Crit: true}, false)
	if !m.has(game.EffectBleed, z.now) {
		t.Error("o crítico deveria sangrar")
	}
	p.pl.cooldowns[game.SkillCutStep] = z.now + 10
	p.pl.cooldowns[game.SkillShadowStrike] = z.now + 10
	m.hp = 1
	p.pl.res.Value = 40
	mercy := game.Skills[game.SkillCutMercy]
	if !castUntil(z, p, mercy.ID, m, func() bool { p.pl.res.Value = 40; return m.dead }) {
		t.Fatal("o Golpe de Misericórdia não matou")
	}
	if _, ok := p.pl.cooldowns[game.SkillCutStep]; ok {
		t.Error("Execução Perfeita deveria zerar o Passo Sombrio")
	}
}

// Disparo Perfurante: acerta até 3 em linha, não o que está fora da linha.
func TestPiercingShot(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 34, 0), fixed("skeleton_warrior", 20, 37, 0),
		fixed("skeleton_warrior", 20, 40, 0), fixed("skeleton_warrior", 20, 42, 0), fixed("skeleton_warrior", 20, 36, 6))
	ms := dummies(z, 34, 37, 40, 42, 36)
	p := evolved(z, 1, "rogue", "hunter", 30)
	n := len(z.frame.combat)
	z.castSkill(p, game.Skills[game.SkillHuntPierce], ms[0])
	hits := hitsSince(z, p, n)
	total := 0
	for _, m := range ms[:4] {
		total += min(1, hits[m.id])
	}
	if total != 3 || hits[ms[4].id] != 0 {
		t.Errorf("Disparo Perfurante: %d em linha (want 3), fora da linha %d", total, hits[ms[4].id])
	}
}

// Armadilha de Caça: o primeiro que pisa leva o golpe e fica preso; ela some.
func TestHuntingTrap(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 31, 0))
	m := dummies(z, 31)[0]
	p := evolved(z, 1, "rogue", "hunter", 30)
	z.castSkill(p, game.Skills[game.SkillHuntTrap], nil)
	hp := m.hp
	z.tickFor(0.5)
	if m.hp >= hp || !m.has(game.EffectRoot, z.now) {
		t.Errorf("armadilha: dano %v, preso %v", hp-m.hp, m.has(game.EffectRoot, z.now))
	}
	z.tickFor(0.5)
	for _, o := range z.objects {
		if o.obj.zone != nil {
			t.Error("a armadilha deveria sumir depois de disparar")
		}
	}
}

// Emboscada, Instinto do Caçador, Perseguição e Mira Perfeita.
func TestHunterHitBonuses(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	p := evolved(z, 1, "rogue", "hunter", 30)
	base := z.hitInput(p, m, false, 1, 0).Mult
	learn(z, p, map[uint32]int{game.SkillHuntAmbush: 1, game.SkillHuntInstinct: 3, game.SkillHuntAim: 1})
	z.applyEffect(p, m, &game.EffectDef{Kind: game.EffectRoot, Duration: 5})
	if got := z.hitInput(p, m, false, 1, 0).Mult; math.Abs(got-base*1.15*1.25) > 1e-9 {
		t.Errorf("preso, Instinto 3 e Emboscada: %v, want %v", got, base*1.15*1.25)
	}
	p.pl.res.Value = 100
	if !z.hitInput(p, m, false, 1, 0).ForceCrit {
		t.Error("Mira Perfeita com a Precisão cheia")
	}
}

// Caçada Implacável: matar o marcado devolve Precisão e passa a marca.
func TestRelentlessHuntPassesMark(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0), fixed("skeleton_warrior", 20, 44, 0))
	ms := dummies(z, 40, 44)
	p := evolved(z, 1, "rogue", "hunter", 30)
	learn(z, p, map[uint32]int{game.SkillHuntRelentless: 1})
	z.castSkill(p, game.Skills[game.SkillHuntPrey], ms[0])
	p.pl.res.Value = 0
	z.now += 2
	z.kill(ms[0], p)
	if p.pl.res.Value < 25 || !ms[1].has(game.EffectMark, z.now) {
		t.Errorf("Caçada Implacável: Precisão %v, marca passou %v", p.pl.res.Value, ms[1].has(game.EffectMark, z.now))
	}
}

// Quem já era Patrulheiro antes do arco: no login as adagas vão para a bolsa e o arco inicial
// entra e é equipado (uma vez).
func TestOldHunterGetsBowOnLogin(t *testing.T) {
	z := newCombatZone()
	p := evolved(z, 1, "rogue", "hunter", 30)
	p.pl.inv.equip[game.SlotWeapon] = stack{}
	delete(p.pl.quests.done, game.StarterItemsFlag("hunter"))
	grantMissingStarterItems(p)
	if p.pl.inv.weaponType() != game.WeaponBow || !p.pl.quests.done[game.StarterItemsFlag("hunter")] {
		t.Fatalf("arma %q", p.pl.inv.weaponType())
	}
	grantMissingStarterItems(p)
	if p.pl.inv.countOf(game.HunterStarterBow) != 0 {
		t.Error("o arco entra uma vez só (já está equipado, não na bolsa)")
	}
}

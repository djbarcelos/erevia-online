package zone

import (
	"math"
	"testing"

	"erevia/server/internal/game"
)

// Testes das mecânicas das evoluções do Cavaleiro (docs/evolucoes/bastion.md e herald.md).

// Guarda Inabalável: +20% de chance de bloqueio enquanto dura (só com escudo).
func TestGuardRaisesBlockChance(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 31, 0))
	m := dummies(z, 31)[0]
	p := evolved(z, 1, "knight", "bastion", 30)
	p.stats.Block = 0.0001 // tem escudo, quase nunca bloqueia
	z.castSkill(p, game.Skills[game.SkillBastionGuard], nil)
	if v, ok := p.effect(game.EffectBuffBlock, z.now); !ok || v != 0.2 {
		t.Fatalf("Guarda: bloqueio %v %v", v, ok)
	}
	blocked := 0
	for range 400 {
		ev := len(z.frame.combat)
		p.hp = p.stats.MaxHP
		z.applyHit(m, p, 0, game.Hit{Damage: 10}, false)
		if z.frame.combat[ev].Blocked {
			blocked++
		}
	}
	if blocked < 50 || blocked > 120 {
		t.Errorf("com a Guarda: %d bloqueios em 400 (≈20%%)", blocked)
	}
}

// Zona de Provocação: a cada 2 s provoca e deixa lento quem está dentro.
func TestTauntZonePulses(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 33, 0), fixed("skeleton_warrior", 20, 45, 0))
	ms := dummies(z, 33, 45)
	p := evolved(z, 1, "knight", "bastion", 30)
	learn(z, p, map[uint32]int{game.SkillBastionZone: 1})
	p.pl.res.Value = 20
	z.castSkill(p, game.Skills[game.SkillBastionZone], nil)
	if p.pl.res.Value != 0 {
		t.Errorf("Zona não gastou 20 de Determinação: %v", p.pl.res.Value)
	}
	z.tickFor(0.3)
	if !ms[0].has(game.EffectTaunt, z.now) || !ms[0].has(game.EffectSlow, z.now) || topHate(ms[0].mon.hate) != p.id {
		t.Error("quem está na zona deveria ser provocado e ficar lento")
	}
	if ms[1].has(game.EffectTaunt, z.now) {
		t.Error("a zona provocou quem está longe")
	}
}

// Eco do Escudo: o Golpe de Escudo acerta até 2 vizinhos a até 3 m, sem atordoar.
func TestShieldEchoHitsTwoNeighbours(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0), fixed("skeleton_warrior", 20, 33, 0),
		fixed("skeleton_warrior", 20, 34, 0), fixed("skeleton_warrior", 20, 34.5, 0))
	ms := dummies(z, 32, 33, 34, 34.5)
	p := evolved(z, 1, "knight", "bastion", 30)
	learn(z, p, map[uint32]int{game.SkillBastionEcho: 3})
	s := game.EffectiveSkill(game.Skills[game.SkillBastionBash], p.pl.ranks)
	n := len(z.frame.combat)
	z.castSkill(p, s, ms[0])
	hits := hitsSince(z, p, n)
	extra := 0
	for _, m := range ms[1:] {
		extra += hits[m.id]
		if m.stunned(z.now) {
			t.Error("o eco não atordoa")
		}
	}
	if hits[ms[0].id] != 1 || extra != 2 {
		t.Errorf("Eco: alvo %d, vizinhos %d (want 2)", hits[ms[0].id], extra)
	}
}

// Interceptar: 30% do dano do aliado vai para o Bastião, sem gerar Determinação em dobro.
func TestInterceptRedirectsDamage(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	b := evolved(z, 1, "knight", "bastion", 30)
	ally := evolved(z, 2, "mage", "elementalist", 32)
	makeParty(z, b, ally)
	learn(z, b, map[uint32]int{game.SkillBastionIntercept: 1})
	z.castSkill(b, game.Skills[game.SkillBastionIntercept], nil)
	if !ally.has(game.EffectGuarded, z.now) || b.has(game.EffectGuarded, z.now) {
		t.Fatal("Interceptar vai só nos aliados")
	}
	bhp, ahp := b.hp, ally.hp
	z.damage(m, ally, 100, nil)
	if got := ahp - ally.hp; got != 70 {
		t.Errorf("o aliado levou %v, want 70", got)
	}
	if got := bhp - b.hp; got != 30 {
		t.Errorf("o Bastião levou %v, want 30", got)
	}
}

// Sacrifício: o aliado não cai abaixo de 25%; o que passar vai para o Paladino.
func TestSacrificeKeepsAllyAboveFloor(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	h := evolved(z, 1, "knight", "herald", 30)
	ally := evolved(z, 2, "rogue", "cutthroat", 32)
	makeParty(z, h, ally)
	h.pl.res.Value = 50
	z.castSkill(h, game.Skills[game.SkillHeraldSacrifice], nil)
	ally.hp = ally.stats.MaxHP * 0.3
	hhp := h.hp
	dmg := ally.stats.MaxHP * 0.2
	z.damage(m, ally, dmg, nil)
	floor := ally.stats.MaxHP * 0.25
	if math.Abs(ally.hp-floor) > 1e-6 {
		t.Errorf("aliado com %v, piso %v", ally.hp, floor)
	}
	if got := hhp - h.hp; math.Abs(got-(dmg-ally.stats.MaxHP*0.05)) > 1e-6 {
		t.Errorf("o Paladino levou %v", got)
	}
}

// Último Reduto: o golpe que mataria deixa com 1 de vida e vira escudo pela Determinação.
func TestLastBastionSavesOnce(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	b := evolved(z, 1, "knight", "bastion", 30)
	learn(z, b, map[uint32]int{game.SkillBastionRedoubt: 1})
	b.pl.res.Value = 50
	z.damage(m, b, b.hp+500, nil)
	if b.dead || b.hp != 1 || b.pl.res.Value != 0 {
		t.Fatalf("Último Reduto: morto %v vida %v recurso %v", b.dead, b.hp, b.pl.res.Value)
	}
	if v, ok := b.effect(game.EffectShield, z.now); !ok || v != math.Round(b.stats.MaxHP*0.5) {
		t.Errorf("escudo %v, want %v", v, math.Round(b.stats.MaxHP*0.5))
	}
	b.removeEffect(game.EffectShield)
	z.damage(m, b, 10, nil)
	if !b.dead {
		t.Error("o Último Reduto tem recarga")
	}
}

// Vínculo de Guarda: o aliado perto do Bastião apanha menos.
func TestGuardLinkCutsAllyDamage(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	b := evolved(z, 1, "knight", "bastion", 30)
	ally := evolved(z, 2, "mage", "elementalist", 33)
	makeParty(z, b, ally)
	learn(z, b, map[uint32]int{game.SkillBastionLink: 5})
	n := len(z.frame.combat)
	z.applyHit(m, ally, 0, game.Hit{Damage: 100}, false)
	if got := z.frame.combat[n].Damage; got != 90 {
		t.Errorf("aliado perto: %d de dano, want 90", got)
	}
}

// Renovação: cura ao longo do tempo em você e nos aliados; gera Fé por aliado ferido.
func TestRenewalHealsOverTimeAndPaysFaith(t *testing.T) {
	z := newCombatZone()
	h := evolved(z, 1, "knight", "herald", 30)
	ally := evolved(z, 2, "rogue", "hunter", 32)
	makeParty(z, h, ally)
	learn(z, h, map[uint32]int{game.SkillHeraldRenewal: 1})
	h.pl.res.Value = 20
	ally.hp = ally.stats.MaxHP / 2
	z.castSkill(h, game.Skills[game.SkillHeraldRenewal], nil)
	if h.pl.res.Value != 4 {
		t.Errorf("Fé depois da Renovação: %v, want 4 (20 gastos, +4 do aliado ferido)", h.pl.res.Value)
	}
	v, ok := ally.effect(game.EffectRegen, z.now)
	if want := ally.stats.MaxHP * 0.08 / 6; !ok || math.Abs(v-want) > 1e-6 {
		t.Fatalf("regeneração %v, want %v", v, want)
	}
	before := ally.hp
	ally.lastCombat = z.now // em combate: só a regeneração normal mais a da Renovação
	z.regen(ally, 1)
	if ally.hp-before < v {
		t.Errorf("curou %v, want ≥ %v", ally.hp-before, v)
	}
}

// Purificar: tira 1 efeito negativo de você e do aliado e cura 3% da vida.
func TestPurifyCleansAndHeals(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	h := evolved(z, 1, "knight", "herald", 30)
	ally := evolved(z, 2, "rogue", "hunter", 32)
	makeParty(z, h, ally)
	z.applyEffect(m, ally, &game.EffectDef{Kind: game.EffectPoison, Duration: 10, Value: 5})
	z.applyEffect(m, h, &game.EffectDef{Kind: game.EffectSlow, Duration: 10, Value: 0.3})
	ally.hp = ally.stats.MaxHP / 2
	before := ally.hp
	z.castSkill(h, game.Skills[game.SkillHeraldPurify], nil)
	if ally.has(game.EffectPoison, z.now) || h.has(game.EffectSlow, z.now) {
		t.Error("Purificar deveria tirar o veneno e a lentidão")
	}
	if got := ally.hp - before; math.Abs(got-math.Round(ally.stats.MaxHP*0.03)) > 1e-6 {
		t.Errorf("cura %v", got)
	}
}

// Bênção Suprema: as bênçãos aprendidas voltam com o efeito dobrado por 8 s.
func TestSupremeBlessingDoubles(t *testing.T) {
	z := newCombatZone()
	h := evolved(z, 1, "knight", "herald", 30)
	learn(z, h, map[uint32]int{game.SkillHeraldSupreme: 1})
	h.pl.res.Value = 60
	z.castSkill(h, game.Skills[game.SkillHeraldSupreme], nil)
	if v, ok := h.effect(game.EffectBuffAtk, z.now); !ok || math.Abs(v-0.2) > 1e-9 {
		t.Errorf("Bênção da Luz dobrada: %v %v", v, ok)
	}
}

// Escudo da Fé: gastar 20 de Fé dá um escudo de 2 × 1% da vida.
func TestFaithShieldOnSpend(t *testing.T) {
	z := newCombatZone()
	h := evolved(z, 1, "knight", "herald", 30)
	learn(z, h, map[uint32]int{game.SkillHeraldFaithWard: 1, game.SkillHeraldRenewal: 1})
	h.pl.res.Value = 20
	z.castSkill(h, game.Skills[game.SkillHeraldRenewal], nil)
	if v, ok := h.effect(game.EffectShield, z.now); !ok || v != math.Round(h.stats.MaxHP*0.01*2) {
		t.Errorf("escudo da Fé %v %v", v, ok)
	}
}

// Zelo e Veredito: as sagradas batem mais; o Veredito, ainda mais em quem está enfraquecido.
func TestHolyDamageBonuses(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	h := evolved(z, 1, "knight", "herald", 30)
	strike := game.Skills[game.SkillHeraldStrike]
	if z.skillDamageBonus(h, m, strike) != 1 {
		t.Fatal("sem passiva, sem bônus")
	}
	learn(z, h, map[uint32]int{game.SkillHeraldZeal: 5, game.SkillHeraldVerdict: 3})
	eff := game.EffectiveSkill(strike, h.pl.ranks)
	if got := z.skillDamageBonus(h, m, eff); math.Abs(got-1.1) > 1e-9 {
		t.Errorf("Zelo 5: %v, want 1,1", got)
	}
	z.applyEffect(h, m, &game.EffectDef{Kind: game.EffectWeaken, Duration: 5, Value: 0.1})
	if got := z.skillDamageBonus(h, m, eff); math.Abs(got-1.1*1.3) > 1e-9 {
		t.Errorf("Veredito 3 em alvo enfraquecido: %v", got)
	}
	if eff.ResOnHit != 8+6 {
		t.Errorf("Veredito: Fé por acerto %v, want 14", eff.ResOnHit)
	}
}

// Prisão do Guardião: prende, provoca e enfraquece quem está a até 8 m.
func TestGuardianPrison(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 34, 0))
	m := dummies(z, 34)[0]
	b := evolved(z, 1, "knight", "bastion", 30)
	b.pl.res.Value = 60
	z.castSkill(b, game.Skills[game.SkillBastionPrison], nil)
	if !m.has(game.EffectRoot, z.now) || !m.has(game.EffectTaunt, z.now) || !m.has(game.EffectWeaken, z.now) || topHate(m.mon.hate) != b.id {
		t.Errorf("Prisão: efeitos %+v", m.effects)
	}
}

// Desafio Reforçado: o raio cresce e os provocados ficam enfraquecidos.
func TestReinforcedDefy(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 39.5, 0))
	m := dummies(z, 39.5)[0]
	b := evolved(z, 1, "knight", "bastion", 30)
	z.castSkill(b, game.Skills[game.SkillBastionDefy], nil)
	if m.has(game.EffectTaunt, z.now) {
		t.Fatal("a 9,5 m o Desafio (7 m) não alcança")
	}
	learn(z, b, map[uint32]int{game.SkillBastionDefyPlus: 3})
	b.pl.cooldowns = map[uint32]float64{}
	z.castSkill(b, game.EffectiveSkill(game.Skills[game.SkillBastionDefy], b.pl.ranks), nil)
	if !m.has(game.EffectTaunt, z.now) || !m.has(game.EffectWeaken, z.now) {
		t.Error("Desafio Reforçado (10 m) deveria provocar e enfraquecer")
	}
}

package zone

import (
	"math"
	"testing"

	"erevia/server/internal/game"
)

// Testes das mecânicas das evoluções do Bárbaro (docs/evolucoes/bloodrager.md e warlord.md).

// Ruptura: sangramento como dano por segundo; Ira Desmedida corta 30% do custo.
func TestRuptureBleedsAndBoundlessCutsCost(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "barbarian", "bloodrager", 30)
	learn(z, p, map[uint32]int{game.SkillRagerRupture: 1})
	rup := game.Skills[game.SkillRagerRupture]
	p.pl.res.Value = 100
	if !castUntil(z, p, rup.ID, m, func() bool {
		ok := m.has(game.EffectBleed, z.now)
		p.pl.res.Value = 100 // a próxima tentativa também tem Fúria
		return ok
	}) {
		t.Fatal("Ruptura não sangrou")
	}
	if !game.EffectBleed.IsDoT() {
		t.Error("sangramento é dano por segundo")
	}
	if game.ResCostFor(rup, p.pl.passives) != 30 {
		t.Fatal("sem a chave, custa 30")
	}
	learn(z, p, map[uint32]int{game.SkillRagerBoundless: 1})
	if got := game.ResCostFor(rup, p.pl.passives); got != 21 {
		t.Errorf("com Ira Desmedida: %v, want 21", got)
	}
}

// Fúria Incontrolável: o básico gera o dobro, e quem lança recebe +10% de dano.
func TestUnchainedFuryDoublesBasicGain(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "barbarian", "bloodrager", 30)
	z.castSkill(p, game.Skills[game.SkillRagerUnchained], nil)
	if !p.has(game.EffectFuryDouble, z.now) || !p.has(game.EffectMark, z.now) {
		t.Fatal("efeitos da Fúria Incontrolável")
	}
	z.applyHit(p, m, 0, game.Hit{Damage: 10}, false)
	if p.pl.res.Value != 12 {
		t.Errorf("básico com Fúria Incontrolável: %v, want 12", p.pl.res.Value)
	}
}

// Sequência Selvagem e Sede Insaciável: velocidade de ataque.
func TestFrenzyStacksAndInsatiable(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0), fixed("skeleton_warrior", 20, 34, 0))
	ms := dummies(z, 32, 34)
	p := evolved(z, 1, "barbarian", "bloodrager", 30)
	learn(z, p, map[uint32]int{game.SkillRagerSequence: 5})
	base := p.attackInterval(z.now)
	for range 7 {
		z.frenzyStack(p, ms[0])
	}
	if p.pl.frenzyStacks != 5 || math.Abs(p.attackInterval(z.now)-base/1.2) > 1e-9 {
		t.Errorf("5 acúmulos de 4%%: intervalo %v, want %v", p.attackInterval(z.now), base/1.2)
	}
	z.frenzyStack(p, ms[1])
	if p.pl.frenzyStacks != 1 {
		t.Errorf("outro alvo recomeça: %d", p.pl.frenzyStacks)
	}
	z.now += game.FrenzyStackSecs + 0.1
	if p.attackInterval(z.now) != base {
		t.Error("depois de 3 s sem bater, os acúmulos somem")
	}
	learn(z, p, map[uint32]int{game.SkillRagerInsatiate: 1})
	p.hp = p.stats.MaxHP * 0.4
	if math.Abs(p.attackInterval(z.now)-base/1.2) > 1e-9 {
		t.Errorf("Sede Insaciável abaixo de 50%%: %v", p.attackInterval(z.now))
	}
}

// Carnificina Final: matar com a Execução zera a recarga (1 vez a cada 10 s); Marca da
// Presa devolve 30 de Fúria no abate.
func TestFinaleResetsAndPreyRefunds(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "barbarian", "bloodrager", 30)
	learn(z, p, map[uint32]int{game.SkillRagerExecute: 1, game.SkillRagerFinale: 1, game.SkillRagerPrey: 1})
	z.castSkill(p, game.Skills[game.SkillRagerPrey], m)
	m.hp = 1
	p.pl.res.Value = 40
	exe := game.Skills[game.SkillRagerExecute]
	if !castUntil(z, p, exe.ID, m, func() bool { return m.dead }) {
		t.Fatal("Execução não matou")
	}
	if _, onCD := p.pl.cooldowns[exe.ID]; onCD {
		t.Error("Carnificina Final deveria zerar a recarga")
	}
	if p.pl.res.Value < 30 {
		t.Errorf("Marca da Presa: Fúria %v depois do abate", p.pl.res.Value)
	}
}

// Liderança: você e os aliados perto causam mais dano, sem somar.
func TestLeadershipAura(t *testing.T) {
	z := newCombatZone()
	w := evolved(z, 1, "barbarian", "warlord", 30)
	ally := evolved(z, 2, "rogue", "cutthroat", 33)
	far := evolved(z, 3, "mage", "occultist", 60)
	makeParty(z, w, ally, far)
	learn(z, w, map[uint32]int{game.SkillWarlordLead: 5})
	if z.leaderAura(w) != 0.05 || z.leaderAura(ally) != 0.05 || z.leaderAura(far) != 0 {
		t.Errorf("Liderança: %v %v %v", z.leaderAura(w), z.leaderAura(ally), z.leaderAura(far))
	}
}

// Grito de Reagrupar: tira lentidão e prender e dá imunidade.
func TestRallyUnstoppable(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	w := evolved(z, 1, "barbarian", "warlord", 30)
	ally := evolved(z, 2, "mage", "elementalist", 32)
	makeParty(z, w, ally)
	z.applyEffect(m, ally, &game.EffectDef{Kind: game.EffectRoot, Duration: 5})
	w.pl.res.Value = 25
	z.castSkill(w, game.Skills[game.SkillWarlordRally], nil)
	if ally.has(game.EffectRoot, z.now) || !ally.has(game.EffectUnstoppable, z.now) {
		t.Fatal("Reagrupar deveria soltar e dar imunidade")
	}
	z.applyEffect(m, ally, &game.EffectDef{Kind: game.EffectSlow, Duration: 5, Value: 0.5})
	if ally.has(game.EffectSlow, z.now) {
		t.Error("imune a lentidão")
	}
}

// Investida Implacável: salta entre até 3 inimigos, sem repetir.
func TestRelentlessChargeChains(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 34, 0), fixed("skeleton_warrior", 20, 38, 0),
		fixed("skeleton_warrior", 20, 42, 0), fixed("skeleton_warrior", 20, 46, 0))
	ms := dummies(z, 34, 38, 42, 46)
	w := evolved(z, 1, "barbarian", "warlord", 30)
	w.pl.res.Value = 60
	n := len(z.frame.combat)
	z.castSkill(w, game.Skills[game.SkillWarlordRelent], ms[0])
	hits := hitsSince(z, w, n)
	if hits[ms[0].id] != 1 || hits[ms[1].id] != 1 || hits[ms[2].id] != 1 || hits[ms[3].id] != 0 {
		t.Errorf("saltos: %v", hits)
	}
}

// Interromper: atordoar durante o aviso no chão cancela o golpe e gera Ímpeto; Presença
// Dominante alonga o atordoamento.
func TestInterruptAndDominance(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	w := evolved(z, 1, "barbarian", "warlord", 30)
	z.pushTelegraph(m, &telegraph{kind: 1, shape: game.ShapeCircle, pos: m.pos, radius: 3}, 2)
	z.applyEffect(w, m, &game.EffectDef{Kind: game.EffectStun, Duration: 1})
	if w.pl.res.Value != 15 {
		t.Errorf("interromper: Ímpeto %v, want 15", w.pl.res.Value)
	}
	z.tickFor(0.1)
	if len(z.telegraphs) != 0 {
		t.Error("o aviso deveria ser cancelado")
	}
	learn(z, w, map[uint32]int{game.SkillWarlordPresence: 1})
	m.effects = nil
	z.applyEffect(w, m, &game.EffectDef{Kind: game.EffectStun, Duration: 1})
	if ef := m.effectRef(game.EffectStun, z.now); ef == nil || math.Abs(ef.until-z.now-1.5) > 1e-9 {
		t.Errorf("atordoamento com Presença: %+v", ef)
	}
	z.tickFor(1.1)
	if !m.has(game.EffectWeaken, z.now) || !m.has(game.EffectSlow, z.now) {
		t.Error("aura da Presença Dominante")
	}
}

// Comando Absoluto: com aliado perto, +50% de Ímpeto e o Brado custa metade da mana.
func TestAbsoluteCommand(t *testing.T) {
	z := newCombatZone()
	w := evolved(z, 1, "barbarian", "warlord", 30)
	ally := evolved(z, 2, "mage", "elementalist", 33)
	learn(z, w, map[uint32]int{game.SkillWarlordCommand2: 1})
	cry := game.Skills[game.SkillWarlordCry]
	if skillMP(w, cry) != math.Round(cry.MP.At(1)*game.CommandMPMult) {
		t.Errorf("Brado com Comando: %v", skillMP(w, cry))
	}
	z.gainRes(w, 10)
	if w.pl.res.Value != 10 {
		t.Fatalf("sem aliado: %v", w.pl.res.Value)
	}
	makeParty(z, w, ally)
	z.now += 2
	z.gainRes(w, 10)
	if w.pl.res.Value != 25 {
		t.Errorf("com aliado: %v, want 25", w.pl.res.Value)
	}
}

// Execução em chefe: no máximo +50%.
func TestExecuteCappedOnBoss(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "barbarian", "bloodrager", 30)
	exe := game.Skills[game.SkillRagerExecute]
	boss := *m.mon.def
	boss.Boss = true
	m.mon.def = &boss
	m.hp = m.stats.MaxHP * 0.1
	m.stats.PDef = 0
	n := len(z.frame.combat)
	z.skillHit(p, m, exe, 1, false)
	ev := z.frame.combat[n]
	if ev.Miss {
		t.Skip("errou")
	}
	in := z.hitInput(p, m, false, exe.Mult.At(1)*game.BossExecuteCap, 0)
	if want := game.ExpectedHit(game.HitInput{Atk: in.Atk, Def: in.Def, Mult: in.Mult, NoMiss: true}); float64(ev.Damage) > want*float64(p.stats.CritDamage)+1 {
		t.Errorf("dano %d acima do teto (%v sem crítico)", ev.Damage, want)
	}
}

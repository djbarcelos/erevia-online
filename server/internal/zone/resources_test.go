package zone

import (
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// evolved: jogador nível 28 já evoluído (árvore da evolução, recurso zerado).
func evolved(z *Zone, id uint32, class, spec string, x float64) *entity {
	p, _ := joinAt(z, id, class, x, 0, game.XPLevelStart(28))
	p.pl.spec = spec
	p.pl.ranks = game.StarterRanks(class, spec)
	z.refreshStats(p)
	return p
}

// Fúria: o ataque básico que acerta gera; o golpe de Fúria é recusado sem recurso e gasta
// quando há; fora de combate a barra cai.
func TestFuryGainSpendAndCalmDecay(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "barbarian", "bloodrager", 30)
	learn(z, p, map[uint32]int{game.SkillRagerRed: 1})

	z.requestSkill(p, game.SkillRagerRed, m.id, false)
	if _, used := p.pl.cooldowns[game.SkillRagerRed]; used {
		t.Fatal("Golpe Rubro sem Fúria")
	}
	hit := game.Hit{Damage: 50}
	z.applyHit(p, m, 0, hit, false)
	if v := p.pl.res.Value; v != 6+15*0 {
		t.Fatalf("básico acertado: Fúria %v, want 6", v)
	}
	// Errar e DoT não geram.
	z.applyHit(p, m, 0, game.Hit{Miss: true}, false)
	z.damage(p, m, 30, nil)
	if p.pl.res.Value != 6 {
		t.Errorf("erro/DoT geraram: %v", p.pl.res.Value)
	}
	// Teto de 25 por segundo.
	for range 10 {
		z.applyHit(p, m, 0, hit, false)
	}
	if p.pl.res.Value > 25+1e-9 {
		t.Errorf("passou do teto por segundo: %v", p.pl.res.Value)
	}
	p.pl.res.Value = 40
	z.castSkill(p, game.EffectiveSkill(game.Skills[game.SkillRagerRed], p.pl.ranks), m)
	if p.pl.res.Value >= 40-20+1e-9 && p.pl.res.Value != 40-20+game.ResourceOf("barbarian", "bloodrager").Gen(game.GenSkillHit) {
		t.Errorf("Golpe Rubro não gastou 20: %v", p.pl.res.Value)
	}
	// Fora de combate: cai 10/s depois de 5 s.
	p.pl.res.Value = 50
	p.lastCombat = z.now - 6
	z.regen(p, 1)
	if p.pl.res.Value != 40 {
		t.Errorf("queda fora de combate: %v", p.pl.res.Value)
	}
}

// Determinação: apanhar de perto e bloquear geram para o Bastião.
func TestDeterminationFromHitsTaken(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 31, 0))
	m := dummies(z, 31)[0]
	p := evolved(z, 1, "knight", "bastion", 30)
	z.applyHit(m, p, 0, game.Hit{Damage: 10}, false)
	if p.pl.res.Value != 1 {
		t.Errorf("golpe de perto: %v, want 1", p.pl.res.Value)
	}
	p.stats.Block = 1 // bloqueia sempre
	z.applyHit(m, p, 0, game.Hit{Damage: 10}, false)
	if p.pl.res.Value != 1+1+6 {
		t.Errorf("bloqueio: %v, want 8", p.pl.res.Value)
	}
	// Guarda Inabalável: +15 ao lançar.
	z.castSkill(p, game.Skills[game.SkillBastionGuard], nil)
	if p.pl.res.Value != 23 {
		t.Errorf("Guarda: %v, want 23", p.pl.res.Value)
	}
}

// Concentração: só gera ao lançar uma skill diferente da última.
func TestConcentrationNeedsDistinctCasts(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 40, 0))
	m := dummies(z, 40)[0]
	p := evolved(z, 1, "mage", "elementalist", 30)
	fire, nova := game.Skills[game.SkillElemFireball], game.Skills[game.SkillElemNova]
	z.castSkill(p, fire, m)
	first := p.pl.res.Value
	z.now += 2 // fora da janela do teto por segundo
	delete(p.pl.cooldowns, fire.ID)
	z.castSkill(p, fire, m)
	if p.pl.res.Value-first > 4+3+1e-9 || first < 10 {
		t.Errorf("repetir a mesma gerou: %v → %v", first, p.pl.res.Value)
	}
	z.now += 2
	before := p.pl.res.Value
	z.castSkill(p, nova, nil)
	if p.pl.res.Value-before < 10 {
		t.Errorf("alternar não gerou: %v → %v", before, p.pl.res.Value)
	}
}

// Precisão: acerto de longe gera pelo degrau; errar zera.
func TestPrecisionRangeAndMiss(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 43, 0))
	m := dummies(z, 43)[0]
	p := evolved(z, 1, "rogue", "hunter", 30)
	z.applyHit(p, m, game.SkillHuntShot, game.Hit{Damage: 10}, false)
	if p.pl.res.Value != 10 {
		t.Fatalf("acerto a 13 m: %v, want 10", p.pl.res.Value)
	}
	z.applyHit(p, m, game.SkillHuntShot, game.Hit{Miss: true}, false)
	if p.pl.res.Value != 0 {
		t.Errorf("errar deveria zerar: %v", p.pl.res.Value)
	}
}

// Combo: pelas costas gera mais; parado 3 s sem acertar, cai 15/s.
func TestComboBackstabAndIdleDecay(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 20, 32, 0))
	m := dummies(z, 32)[0]
	p := evolved(z, 1, "rogue", "cutthroat", 30)
	m.dest = Vec2{X: 40, Z: 0} // o monstro olha para +X: o ladino em x=30 está atrás
	z.applyHit(p, m, 0, game.Hit{Damage: 10}, false)
	if p.pl.res.Value != 10 {
		t.Errorf("básico pelas costas: %v, want 10", p.pl.res.Value)
	}
	m.dest = Vec2{X: 20, Z: 0} // agora olha para o ladino
	z.now += 2
	z.applyHit(p, m, 0, game.Hit{Damage: 10}, false)
	if p.pl.res.Value != 15 {
		t.Errorf("básico de frente: %v, want 15", p.pl.res.Value)
	}
	z.now += 4
	z.regen(p, 1)
	if p.pl.res.Value != 0 && p.pl.res.Value != 15-15 {
		t.Errorf("Combo parado: %v", p.pl.res.Value)
	}
}

// Trocar de evolução ou de subclasse zera; o SelfStats leva o valor.
func TestResourceResetsAndIsSent(t *testing.T) {
	z := newCombatZone()
	p, peer := nearMaster(z, 1, "knight", 25)
	p.pl.spec = "bastion"
	p.pl.ranks = game.StarterRanks("knight", "bastion")
	p.pl.res.Value = 42
	p.lastCombat = z.now // em combate: sem a queda de fora de combate
	z.tickFor(0.2)
	if s := peer.lastSelfStats(); s == nil || s.Res != 42 {
		t.Fatalf("SelfStats.res: %+v", s)
	}
	p.pl.inv.gold = game.EvolutionChangeCost(25)
	specCmd(z, p, pb.SpecCmdKind_SPEC_CMD_CHOOSE, "herald")
	if p.pl.spec != "herald" || p.pl.res.Value != 0 {
		t.Errorf("trocar de evolução: %q %v", p.pl.spec, p.pl.res.Value)
	}
	if c := specCatalog(); c[0].Resource == nil || c[0].Resource.Name != "Determinação" || c[0].Resource.Max != 100 {
		t.Errorf("catálogo sem o recurso: %+v", c[0].Resource)
	}
}

func (f *fakePeer) lastSelfStats() *pb.SelfStats {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.msgs) - 1; i >= 0; i-- {
		if s := f.msgs[i].GetSelfStats(); s != nil {
			return s
		}
	}
	return nil
}

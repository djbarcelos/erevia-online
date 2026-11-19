package zone

import (
	"testing"

	"erevia/server/internal/game"
)

// Fase 4 (tunnel#180): o escudo dá chance de bloqueio pelo grau, o bloqueio corta metade do
// golpe físico e magia nunca é bloqueada.
func TestShieldBlocksPhysicalHitsOnly(t *testing.T) {
	if b := game.Items["shield_badge"].Bonus.Block; b != game.ShieldBlock(game.GradeCommon) || b <= 0 {
		t.Fatalf("escudo comum com bloqueio %v", b)
	}
	z := newCombatZone(fixed("skeleton_warrior", 3, 31, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", 30, 0, 0)
	p.stats.Block = 1 // sempre bloqueia, para o teste não depender do sorteio
	p.stats.MaxHP, p.hp = 10000, 10000
	z.applyHit(m, p, 0, game.Hit{Damage: 100}, false)
	if lost := 10000 - p.hp; lost != 100*(1-game.BlockReduceBase) {
		t.Fatalf("golpe bloqueado tirou %v de vida", lost)
	}
	ev := z.frame.combat[len(z.frame.combat)-1]
	if !ev.Blocked || ev.Damage != 50 {
		t.Fatalf("evento do bloqueio: %+v", ev)
	}
	hp := p.hp
	z.applyHit(m, p, 0, game.Hit{Damage: 100}, true) // magia
	if hp-p.hp != 100 || z.frame.combat[len(z.frame.combat)-1].Blocked {
		t.Fatal("magia não pode ser bloqueada")
	}
}

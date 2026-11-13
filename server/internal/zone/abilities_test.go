package zone

import (
	"math"
	"testing"

	"erevia/server/internal/game"
	"erevia/server/internal/pb"
)

// shapeAt monta um aviso como o abilityAt monta (cone e linha apontados para +X).
func shapeAt(a game.AbilityDef) *telegraph {
	t := &telegraph{shape: a.Shape, radius: a.Radius, inner: a.Inner, length: a.Length, width: a.Width}
	t.aim(Vec2{X: 1})
	t.arc = a.Arc * math.Pi / 180
	t.cosHalf = math.Cos(t.arc / 2)
	return t
}

func TestTelegraphShapesEdges(t *testing.T) {
	const eps = 0.02
	at := func(r, deg float64) Vec2 { // deg a partir de +X (a direção do aviso)
		a := deg * math.Pi / 180
		return Vec2{X: r * math.Cos(a), Z: r * math.Sin(a)}
	}
	cases := []struct {
		name string
		def  game.AbilityDef
		in   []Vec2
		out  []Vec2
	}{
		{"círculo: borda", game.AbilityDef{Shape: game.ShapeCircle, Radius: 4},
			[]Vec2{{}, at(4-eps, 0), at(4-eps, 137), at(4, 90)},
			[]Vec2{at(4+eps, 0), at(4+eps, 250)}},
		{"cone: raio e ângulo", game.AbilityDef{Shape: game.ShapeCone, Radius: 5, Arc: 90},
			[]Vec2{{}, at(0.5, 0), at(5-eps, 0), at(3, 45-0.5), at(3, -45+0.5), at(5-eps, 44)},
			[]Vec2{at(5+eps, 0), at(3, 45+0.5), at(3, -45-0.5), at(2, 180), at(2, 90)}},
		{"cone estreito de 30°", game.AbilityDef{Shape: game.ShapeCone, Radius: 8, Arc: 30},
			[]Vec2{at(6, 14.5), at(6, -14.5)},
			[]Vec2{at(6, 15.5), at(6, -15.5)}},
		{"linha: comprimento e largura", game.AbilityDef{Shape: game.ShapeLine, Length: 10, Width: 2},
			[]Vec2{{}, {X: 10 - eps}, {X: 5, Z: 1 - eps}, {X: 5, Z: -1 + eps}, {X: 10 - eps, Z: 1 - eps}},
			[]Vec2{{X: 10 + eps}, {X: -eps}, {X: 5, Z: 1 + eps}, {X: 5, Z: -1 - eps}, {X: -1, Z: 0.5}}},
		{"anel: raio interno e externo", game.AbilityDef{Shape: game.ShapeDonut, Inner: 3, Radius: 9},
			[]Vec2{at(3+eps, 0), at(6, 200), at(9-eps, 77)},
			[]Vec2{{}, at(3-eps, 0), at(1, 120), at(9+eps, 300)}},
	}
	for _, c := range cases {
		tg := shapeAt(c.def)
		for _, p := range c.in {
			if !tg.contains(p) {
				t.Errorf("%s: (%.2f, %.2f) deveria estar dentro", c.name, p.X, p.Z)
			}
		}
		for _, p := range c.out {
			if tg.contains(p) {
				t.Errorf("%s: (%.2f, %.2f) deveria estar fora", c.name, p.X, p.Z)
			}
		}
	}
}

func TestTelegraphAimFollowsDirection(t *testing.T) {
	// Cone apontado para o sul (-Z): quem está ao sul leva, quem está ao norte não.
	tg := &telegraph{shape: game.ShapeCone, radius: 5, pos: Vec2{X: 40, Z: 0}}
	tg.aim(Vec2{X: 40, Z: -3})
	tg.cosHalf = math.Cos(math.Pi / 4)
	if math.Abs(tg.angle-math.Pi) > 1e-9 && math.Abs(tg.angle+math.Pi) > 1e-9 {
		t.Fatalf("angle = %v, want ±π", tg.angle)
	}
	if !tg.contains(Vec2{X: 40, Z: -4}) || tg.contains(Vec2{X: 40, Z: 4}) {
		t.Fatal("the cone points where it aimed")
	}
	// Linha para o leste (+X): o ângulo é π/2, como o rotation.y do Babylon.
	ln := &telegraph{shape: game.ShapeLine, length: 6, width: 1, pos: Vec2{}}
	ln.aim(Vec2{X: 2})
	if math.Abs(ln.angle-math.Pi/2) > 1e-9 || !ln.contains(Vec2{X: 5.9}) || ln.contains(Vec2{Z: 2}) {
		t.Fatalf("line east: angle=%v", ln.angle)
	}
}

// abilityHits: os golpes de habilidade (não o ataque básico) que p recebeu.
func abilityHits(peer *fakePeer, p *entity) int {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	n := 0
	for _, m := range peer.msgs {
		for _, c := range m.GetSnapshot().GetCombat() {
			if c.TargetId == p.id && c.SkillId == game.HitMonsterAbility {
				n++
			}
		}
	}
	return n
}

// pending: o aviso pendente de m (nil = nenhum).
func (z *Zone) pendingFrom(m *entity) *telegraph {
	for _, t := range z.telegraphs {
		if t.src == m.id {
			return t
		}
	}
	return nil
}

// untilLastTick avança até o tick que vai resolver t (sem rodá-lo).
func (z *Zone) untilLastTick(t *telegraph) {
	dt := 1 / float64(z.cfg.TickRate)
	for z.now+dt < t.at {
		z.update(dt)
	}
}

// golemFight: um golem brigando com um jogador parado bem na frente dele.
func golemFight(t *testing.T) (*Zone, *entity, *entity, *fakePeer) {
	t.Helper()
	z := newCombatZone(fixed("stone_golem", 25, 40, 0))
	m := z.onlyMonster()
	p, peer := joinAt(z, 1, "knight", m.pos.X+2.5, m.pos.Z, 0)
	atLevel(z, p, 25)
	m.mon.hate[p.id] = 1
	return z, m, p, peer
}

func TestGolemSlamWarnsThenHitsWhoStaysInTheCone(t *testing.T) {
	z, m, p, peer := golemFight(t)
	slam := &m.mon.def.Abilities[0]
	z.tickFor(slam.First - 0.5)
	if z.pendingFrom(m) != nil {
		t.Fatal("the first slam waits First seconds after the fight starts")
	}
	z.tickFor(0.6)
	tg := z.pendingFrom(m)
	if tg == nil || tg.shape != game.ShapeCone || tg.kind != pb.TelegraphKind_TELEGRAPH_KIND_STRIKE {
		t.Fatalf("the golem should be winding up a cone: %+v", tg)
	}
	if math.Abs(tg.at-z.now-slam.Delay) > 0.06 {
		t.Fatalf("the hit lands %.2f s after the warning, want %.2f", tg.at-z.now, slam.Delay)
	}
	if !peer.has(func(msg *pb.ServerMessage) bool {
		for _, w := range msg.GetSnapshot().GetTelegraphs() {
			if w.Shape == pb.TelegraphShape_TELEGRAPH_SHAPE_CONE && w.SourceId == m.id && w.DurationMs > 0 && w.Arc > 1.5 && w.Arc < 1.6 {
				return true
			}
		}
		return false
	}) {
		t.Fatal("the client gets the cone with its direction and opening")
	}
	z.untilLastTick(tg)
	z.tickFor(0.05)
	if abilityHits(peer, p) != 1 || !p.has(game.EffectSlow, z.now) {
		t.Fatalf("whoever stays in the cone takes the slam and the slow (hits=%d)", abilityHits(peer, p))
	}
}

func TestPlayerLeavingOnTheLastTickDodges(t *testing.T) {
	z, m, p, peer := golemFight(t)
	z.tickFor(m.mon.def.Abilities[0].First + 0.1)
	tg := z.pendingFrom(m)
	if tg == nil {
		t.Fatal("no slam")
	}
	z.untilLastTick(tg)
	// Sai pela lateral do cone de 90° no último tick antes de o golpe cair.
	p.teleportTo(Vec2{X: tg.pos.X, Z: tg.pos.Z + 3})
	z.tickFor(0.05)
	if z.pendingFrom(m) != nil {
		t.Fatal("the slam should have resolved")
	}
	if abilityHits(peer, p) != 0 {
		t.Fatal("leaving on the last tick dodges the hit")
	}
}

func TestAbilityCooldown(t *testing.T) {
	z, m, p, _ := golemFight(t)
	slam := m.mon.def.Abilities[0]
	p.stats.MaxHP, p.hp = 1e7, 1e7 // aguenta a luta inteira
	starts := 0
	var times []float64
	seen := map[uint32]bool{}
	for range int((slam.First + 3*slam.Cooldown + 0.5) * float64(z.cfg.TickRate)) {
		z.update(1 / float64(z.cfg.TickRate))
		for _, tg := range z.telegraphs {
			if !seen[tg.id] {
				seen[tg.id] = true
				starts++
				times = append(times, z.now)
			}
		}
	}
	if starts != 4 {
		t.Fatalf("First + 3 cooldowns: 4 slams, got %d at %v", starts, times)
	}
	for i := 1; i < len(times); i++ {
		if d := times[i] - times[i-1]; math.Abs(d-slam.Cooldown) > 0.06 {
			t.Fatalf("slams %.2f s apart, want %.0f", d, slam.Cooldown)
		}
	}
}

func TestAbilityCancelledWhenMonsterResets(t *testing.T) {
	z, m, p, peer := golemFight(t)
	z.tickFor(m.mon.def.Abilities[0].First + 0.1)
	if z.pendingFrom(m) == nil {
		t.Fatal("no slam")
	}
	z.resetMonster(m)
	z.tickFor(2)
	if abilityHits(peer, p) != 0 || len(z.telegraphs) != 0 {
		t.Fatal("a monster that gives up cancels its pending hits")
	}
	// A próxima luta espera o First de novo.
	m.mon.returning = false
	m.mon.hate[p.id] = 1
	z.tickFor(1)
	if z.pendingFrom(m) != nil {
		t.Fatal("a new fight starts the abilities from First")
	}
}

func TestBoneLordDonutSparesWhoHugsIt(t *testing.T) {
	z := newCombatZone(fixed("bone_lord", 22, 60, 0))
	m := z.onlyMonster()
	hug, hugPeer := joinAt(z, 1, "knight", m.pos.X+1.5, m.pos.Z, 0)
	ranged, rangedPeer := joinAt(z, 2, "mage", m.pos.X+6, m.pos.Z, 0)
	for _, p := range []*entity{hug, ranged} {
		atLevel(z, p, 22)
		p.stats.MaxHP, p.hp = 1e7, 1e7
	}
	m.mon.hate[hug.id] = 10
	m.mon.hate[ranged.id] = 1
	ring := m.mon.def.Abilities[0]
	z.tickFor(ring.First + 0.1)
	tg := z.pendingFrom(m)
	if tg == nil || tg.shape != game.ShapeDonut {
		t.Fatal("the bone lord raises the ring")
	}
	hug.teleportTo(Vec2{X: m.pos.X + 1.5, Z: m.pos.Z})
	z.untilLastTick(tg)
	z.tickFor(0.05)
	if abilityHits(hugPeer, hug) != 0 || abilityHits(rangedPeer, ranged) != 1 {
		t.Fatalf("donut: hug=%d ranged=%d", abilityHits(hugPeer, hug), abilityHits(rangedPeer, ranged))
	}
}

func TestTelegraphOnlyReachesTheAreaOfInterest(t *testing.T) {
	z, m, _, nearPeer := golemFight(t)
	_, farPeer := joinAt(z, 2, "mage", m.pos.X-aoiRadius-aoiSlack-20, m.pos.Z, 0)
	z.tickFor(m.mon.def.Abilities[0].First + 0.1)
	got := func(peer *fakePeer) bool {
		return peer.has(func(msg *pb.ServerMessage) bool { return len(msg.GetSnapshot().GetTelegraphs()) > 0 })
	}
	if !got(nearPeer) {
		t.Fatal("who is near sees the warning")
	}
	if got(farPeer) {
		t.Fatal("the warning is not broadcast to the whole map")
	}
}

func TestAimCurrentAndLineFromTheMonster(t *testing.T) {
	z := newCombatZone(fixed("skeleton_warrior", 10, 40, 0))
	m := z.onlyMonster()
	p, _ := joinAt(z, 1, "knight", m.pos.X, m.pos.Z+3, 0)
	m.act = action{kind: actAttack, target: p.id}
	z.castAbility(m, &game.AbilityDef{Shape: game.ShapeCircle, Target: game.AimCurrent, Radius: 2, Delay: 1})
	z.castAbility(m, &game.AbilityDef{Shape: game.ShapeLine, Target: game.AimCurrent, Length: 8, Width: 2, Delay: 1})
	if len(z.telegraphs) != 2 {
		t.Fatalf("got %d warnings", len(z.telegraphs))
	}
	c, l := z.telegraphs[0], z.telegraphs[1]
	if c.pos != p.pos || l.pos != m.pos || !l.contains(p.pos) || l.contains(Vec2{X: m.pos.X, Z: m.pos.Z - 3}) {
		t.Fatal("circles land on the target; lines leave the monster toward it")
	}
}

func TestRaidAbilitiesKeepTheOldNumbers(t *testing.T) {
	// Os números do raid antes de virarem dados (raid.go até a tunnel#130).
	want := map[string]struct {
		every, radius, delay, mult, first float64
		phase, count                      int
		hit                               uint32
	}{
		"wave": {14, 8, 2.5, 1.6, 8, 1, 0, RaidHitWave},
		"rain": {12, 4, 2.0, 1.0, 3, 2, 3, RaidHitRain},
		"mark": {16, 5, 5.0, 1.4, 4, 3, 0, RaidHitMark},
	}
	ab := game.Monsters["ozrhal"].Abilities
	if len(ab) != 3 || ab[0].ID != "wave" || ab[1].ID != "rain" || ab[2].ID != "mark" {
		t.Fatal("Ozrhal checks wave, rain and mark, in this order")
	}
	for _, a := range ab {
		w := want[a.ID]
		if a.Cooldown != w.every || a.Radius != w.radius || a.Delay != w.delay || a.Mult != w.mult || a.First != w.first ||
			a.MinPhase != w.phase || a.Count != w.count || a.Hit() != w.hit || a.Shape != game.ShapeCircle {
			t.Errorf("%s changed: %+v", a.ID, a)
		}
	}
	if e := ab[0].Effect; e == nil || e.Kind != game.EffectSlow || e.Duration != 3 || e.Value != 0.4 {
		t.Error("the wave slows 40% for 3 s")
	}
	if e := ab[1].Effect; e == nil || e.Kind != game.EffectStun || e.Duration != 1 {
		t.Error("the rain stuns for 1 s")
	}
	if e := ab[2].OnMarked; ab[2].Effect != nil || e == nil || e.Kind != game.EffectDeathMark || e.Duration != raidMarkDelay {
		t.Error("the mark puts the death mark on the marked one and nothing on who is hit")
	}
}

func TestRaidScheduleKeepsTheOldTimings(t *testing.T) {
	z, m := newRaidZone(nil)
	tank, _ := raider(z, 1, "knight", raidX-3, raidZ)
	dps, _ := raider(z, 2, "mage", raidX-10, raidZ)
	m.mon.hate[tank.id] = 100
	m.mon.hate[dps.id] = 10
	z.think(m)
	r := m.mon.raid
	if r.nextWave != z.now+8 {
		t.Fatalf("first wave 8 s after engaging: %.2f", r.nextWave-z.now)
	}
	r.nextWave = z.now
	z.think(m)
	if r.nextWave != z.now+14 || len(z.telegraphs) != 1 {
		t.Fatal("the wave comes back every 14 s")
	}
	m.hp = m.stats.MaxHP * 0.69
	z.think(m)
	if r.nextRain != z.now+3 {
		t.Fatalf("phase 2: rain 3 s later, got %.2f", r.nextRain-z.now)
	}
	m.hp = m.stats.MaxHP * 0.39
	z.think(m)
	if r.nextMark != z.now+4 {
		t.Fatalf("phase 3: mark 4 s later, got %.2f", r.nextMark-z.now)
	}
	r.nextRain, r.nextMark = z.now, z.now
	z.think(m)
	if r.nextRain != z.now+12 || r.nextMark != z.now+16 {
		t.Fatal("rain every 12 s, mark every 16 s")
	}
}

func TestDevWarningsOnlyInDevAndHarmless(t *testing.T) {
	z := newCombatZone(fixed("skeleton_minion", 1, 40, 0))
	p, peer := joinAt(z, 1, "knight", 30, 0, 0)
	z.handle(chatEvent{id: p.id, text: "/avisos"})
	if len(z.telegraphs) != 0 {
		t.Fatal("/avisos must do nothing without DevCommands (production)")
	}
	z.cfg.DevCommands = true
	p.pl.chatTokens = 10
	z.handle(chatEvent{id: p.id, text: "/avisos"})
	if len(z.telegraphs) != 10 {
		t.Fatalf("/avisos makes 10 warnings, got %d", len(z.telegraphs))
	}
	hp := p.hp
	z.tickFor(5)
	if len(z.telegraphs) != 0 || p.hp != hp || abilityHits(peer, p) != 0 {
		t.Fatal("test warnings resolve without hurting anyone")
	}
}
